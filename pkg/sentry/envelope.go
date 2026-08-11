package sentry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Envelope is a parsed Sentry envelope: headers plus items.
type Envelope struct {
	Headers json.RawMessage
	Items   []EnvelopeItem
}

// EnvelopeItem is a single envelope item (header + raw payload bytes).
// Payload is nil if missing. For event items the bytes are JSON, whether the
// wire form was newline-delimited JSON or length-prefixed ("binary") payload.
type EnvelopeItem struct {
	Header  map[string]interface{}
	Payload json.RawMessage // nil if missing
}

// FilterOutBinaryItems strips replay / binary lines that would crash envelope parsing.
// Port of workers/sentry filterOutBinaryItems — keep behavior identical.
// Prefer ParseEnvelope for full envelopes: it is length-aware and skips replays itself.
func FilterOutBinaryItems(rawEvent string) string {
	lines := strings.Split(rawEvent, "\n")
	filteredLines := make([]string, 0, len(lines))
	isInReplayBlock := false

	for i, line := range lines {
		// Keep envelope header (first line)
		if i == 0 {
			filteredLines = append(filteredLines, line)
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			// If line doesn't parse as JSON, it is likely an item payload (e.g., attachment/binary).
			// Keep it unless we're inside a replay block (replay recordings can include raw binary/newlines).
			if isInReplayBlock {
				continue
			}
			filteredLines = append(filteredLines, line)
			continue
		}

		itemType, _ := parsed["type"].(string)

		// Check if this is a replay event type
		if itemType == "replay_recording" || itemType == "replay_event" {
			isInReplayBlock = true
			continue
		}

		// If we're in a replay block, check if this is still part of it
		if isInReplayBlock {
			_, hasSegmentID := parsed["segment_id"]
			_, hasLength := parsed["length"]
			_, hasReplayID := parsed["replay_id"]

			if hasSegmentID || (hasLength && itemType != "event") || hasReplayID {
				continue
			}

			// If it's a new envelope item (like event), we've exited the replay block
			if itemType == "event" || itemType == "transaction" || itemType == "session" {
				isInReplayBlock = false
			} else {
				// Unknown type, assume we're still in replay block
				continue
			}
		}

		// Keep valid headers and other JSON data (not in replay block)
		if !isInReplayBlock {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.Join(filteredLines, "\n")
}

// ParseEnvelope parses a Sentry envelope.
// Item payloads support both wire forms used by Sentry SDKs:
//   - newline-delimited JSON (no "length" in the item header)
//   - length-prefixed bytes (item header has numeric "length")
// Replay items are skipped (same outcome as FilterOutBinaryItems).
func ParseEnvelope(raw []byte) (*Envelope, error) {
	data := skipLeadingNewlines(raw)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty envelope")
	}

	headerBytes, rest, err := readLine(data)
	if err != nil || len(bytes.TrimSpace(headerBytes)) == 0 {
		return nil, fmt.Errorf("empty envelope")
	}
	if !json.Valid(headerBytes) {
		return nil, fmt.Errorf("failed to parse envelope header: invalid JSON")
	}

	items := make([]EnvelopeItem, 0)
	isInReplayBlock := false

	for len(bytes.TrimSpace(rest)) > 0 {
		rest = skipLeadingNewlines(rest)
		if len(rest) == 0 {
			break
		}

		headerLine, next, lineErr := readLine(rest)
		if lineErr != nil {
			return nil, fmt.Errorf("failed to read item header: %w", lineErr)
		}

		var itemHeader map[string]interface{}
		if err := json.Unmarshal(headerLine, &itemHeader); err != nil {
			// Non-JSON line: treat like FilterOutBinaryItems (skip in replay, else error)
			if isInReplayBlock {
				rest = next
				continue
			}
			return nil, fmt.Errorf("failed to parse item header: %w", err)
		}
		rest = next

		itemType, _ := itemHeader["type"].(string)
		skip := false

		if itemType == "replay_recording" || itemType == "replay_event" {
			isInReplayBlock = true
			skip = true
		} else if isInReplayBlock {
			_, hasSegmentID := itemHeader["segment_id"]
			_, hasLength := itemHeader["length"]
			_, hasReplayID := itemHeader["replay_id"]

			if hasSegmentID || (hasLength && itemType != "event") || hasReplayID {
				skip = true
			} else if itemType == "event" || itemType == "transaction" || itemType == "session" {
				isInReplayBlock = false
			} else {
				skip = true
			}
		}

		if skip {
			// Replay/binary stubs in tests often declare a large length but only
			// include a short payload — consume what we can and continue.
			rest = consumeItemPayloadBestEffort(rest, itemHeader)
			continue
		}

		payload, next, err := readItemPayload(rest, itemHeader)
		if err != nil {
			return nil, err
		}
		rest = next

		items = append(items, EnvelopeItem{
			Header:  itemHeader,
			Payload: payload,
		})
	}

	return &Envelope{
		Headers: json.RawMessage(headerBytes),
		Items:   items,
	}, nil
}

// readItemPayload reads the payload for one envelope item.
// If the header has a numeric "length", exactly that many bytes are taken
// (length-prefixed / "binary" form). Otherwise the next newline-terminated
// line is taken as JSON.
func readItemPayload(data []byte, header map[string]interface{}) (json.RawMessage, []byte, error) {
	if len(data) == 0 {
		return nil, data, nil
	}

	if n, ok := headerLength(header); ok {
		if n < 0 {
			return nil, nil, fmt.Errorf("invalid item payload length: %d", n)
		}
		if len(data) < n {
			return nil, nil, fmt.Errorf("item payload shorter than declared length: need %d, have %d", n, len(data))
		}
		payload := data[:n]
		rest := data[n:]
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		}
		return json.RawMessage(payload), rest, nil
	}

	line, rest, err := readLine(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read item payload: %w", err)
	}
	if len(line) == 0 {
		return nil, rest, nil
	}
	return json.RawMessage(line), rest, nil
}

// consumeItemPayloadBestEffort advances past an item payload without failing when
// the declared length exceeds the remaining buffer (common for truncated replay stubs).
func consumeItemPayloadBestEffort(data []byte, header map[string]interface{}) []byte {
	if len(data) == 0 {
		return data
	}
	if n, ok := headerLength(header); ok && n >= 0 {
		if len(data) < n {
			return nil
		}
		rest := data[n:]
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		}
		return rest
	}
	_, rest, err := readLine(data)
	if err != nil {
		return nil
	}
	return rest
}

func headerLength(header map[string]interface{}) (int, bool) {
	if header == nil {
		return 0, false
	}
	raw, ok := header["length"]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return int(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

func readLine(data []byte) ([]byte, []byte, error) {
	if len(data) == 0 {
		return nil, data, fmt.Errorf("unexpected end of envelope")
	}
	idx := bytes.IndexByte(data, '\n')
	if idx < 0 {
		return data, nil, nil
	}
	return data[:idx], data[idx+1:], nil
}

func skipLeadingNewlines(data []byte) []byte {
	for len(data) > 0 && (data[0] == '\n' || data[0] == '\r') {
		data = data[1:]
	}
	return data
}

// ItemType returns the envelope item type from its header.
func (item EnvelopeItem) ItemType() string {
	if item.Header == nil {
		return ""
	}
	t, _ := item.Header["type"].(string)
	return t
}
