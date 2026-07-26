package sentry

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Envelope is a parsed Sentry envelope: headers plus items.
type Envelope struct {
	Headers json.RawMessage
	Items   []EnvelopeItem
}

// EnvelopeItem is a single envelope item (header + raw JSON payload).
type EnvelopeItem struct {
	Header  map[string]interface{}
	Payload json.RawMessage // nil if missing
}

// FilterOutBinaryItems strips replay / binary lines that would crash envelope parsing.
// Port of workers/sentry filterOutBinaryItems — keep behavior identical.
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
			// If line doesn't parse as JSON, it might be binary data
			// If we're in a replay block, skip it (it's part of replay recording)
			if isInReplayBlock {
				continue
			}
			// If not in replay block and not JSON, it might be corrupted data - skip it
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

// ParseEnvelope parses a newline-delimited Sentry envelope after binary filtering.
func ParseEnvelope(raw []byte) (*Envelope, error) {
	filtered := FilterOutBinaryItems(string(raw))
	lines := splitNonEmptyLines(filtered)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty envelope")
	}

	if !json.Valid([]byte(lines[0])) {
		return nil, fmt.Errorf("failed to parse envelope header: invalid JSON")
	}

	items := make([]EnvelopeItem, 0)
	for i := 1; i < len(lines); {
		var itemHeader map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i]), &itemHeader); err != nil {
			return nil, fmt.Errorf("failed to parse item header at line %d: %w", i, err)
		}
		i++

		var payload json.RawMessage
		if i < len(lines) {
			rawPayload := []byte(lines[i])
			if !json.Valid(rawPayload) {
				return nil, fmt.Errorf("failed to parse item payload at line %d: invalid JSON", i)
			}
			payload = json.RawMessage(rawPayload)
			i++
		}

		items = append(items, EnvelopeItem{
			Header:  itemHeader,
			Payload: payload,
		})
	}

	return &Envelope{
		Headers: json.RawMessage(lines[0]),
		Items:   items,
	}, nil
}

func splitNonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// ItemType returns the envelope item type from its header.
func (item EnvelopeItem) ItemType() string {
	if item.Header == nil {
		return ""
	}
	t, _ := item.Header["type"].(string)
	return t
}
