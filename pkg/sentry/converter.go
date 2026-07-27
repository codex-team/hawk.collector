package sentry

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// CatcherVersion matches hawk-worker-sentry package.json version (parity).
const CatcherVersion = "1.0.1"

// Queue / catcher type constants matching Node worker routing.
const (
	CatcherTypeJavaScript = "errors/javascript"
	CatcherTypeDefault    = "errors/default"
)

// sentryJsSDK list from Node worker (including capacirtor typo — do not "fix").
var sentryJsSDK = []string{"browser", "react", "vue", "angular", "capacirtor", "electron"}

// addonFields are Sentry event fields copied into addons.sentry (same order as Node).
var addonFields = []string{
	"message",
	"logentry",
	"timestamp",
	"start_timestamp",
	"level",
	"platform",
	"server_name",
	"dist",
	"environment",
	"request",
	"transaction",
	"modules",
	"fingerprint",
	"tags",
	"extra",
}

// SourceCodeLine is a single line of inline source context.
type SourceCodeLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// BacktraceFrame is a Hawk backtrace frame.
type BacktraceFrame struct {
	File       string           `json:"file"`
	Line       int              `json:"line"`
	Column     *int             `json:"column,omitempty"`
	Function   string           `json:"function,omitempty"`
	Arguments  []string         `json:"arguments,omitempty"`
	SourceCode []SourceCodeLine `json:"sourceCode,omitempty"`
}

// HawkUser is a Hawk user object.
type HawkUser struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// HawkEventPayload is the Hawk catcher payload produced from a Sentry event.
type HawkEventPayload struct {
	Title          string                 `json:"title"`
	Type           string                 `json:"type"`
	CatcherVersion string                 `json:"catcherVersion"`
	Backtrace      []BacktraceFrame       `json:"backtrace,omitempty"`
	Context        json.RawMessage        `json:"context,omitempty"`
	User           *HawkUser              `json:"user,omitempty"`
	Release        string                 `json:"release,omitempty"`
	Addons         map[string]interface{} `json:"addons,omitempty"`
}

// HawkBrokerPayload is the message body published to errors/* queues.
type HawkBrokerPayload struct {
	ProjectID   string           `json:"projectId"`
	CatcherType string           `json:"catcherType"`
	Timestamp   int64            `json:"timestamp"`
	Payload     HawkEventPayload `json:"payload"`
}

// ComposeTitle builds Hawk title from a Sentry event payload.
func ComposeTitle(eventPayload gjson.Result) string {
	exception := eventPayload.Get("exception.values.0")
	if exception.Exists() {
		excType := exception.Get("type").String()
		if excType == "" {
			excType = "Unknown"
		}
		return fmt.Sprintf("%s: %s", excType, exception.Get("value").String())
	}

	message := eventPayload.Get("message")
	if message.Exists() && message.Type != gjson.Null {
		return message.String()
	}

	return "Unknown: "
}

// ComposeBacktrace builds Hawk backtrace from the first exception's stacktrace.
func ComposeBacktrace(eventPayload gjson.Result) []BacktraceFrame {
	frames := eventPayload.Get("exception.values.0.stacktrace.frames")
	if !frames.Exists() || !frames.IsArray() {
		return nil
	}

	rawFrames := frames.Array()
	if len(rawFrames) == 0 {
		return nil
	}

	// Sentry sends backtrace in reverse order — reverse to get correct order
	reversed := make([]gjson.Result, len(rawFrames))
	for i, frame := range rawFrames {
		reversed[len(rawFrames)-1-i] = frame
	}

	backtrace := make([]BacktraceFrame, 0, len(reversed))
	for _, frame := range reversed {
		file := firstNonEmpty(
			frame.Get("filename").String(),
			frame.Get("abs_path").String(),
			frame.Get("module").String(),
			frame.Get("instruction_addr").String(),
			"unknown location",
		)

		line := int(frame.Get("lineno").Int())
		bt := BacktraceFrame{
			File: file,
			Line: line,
		}

		hasContextLine := frame.Get("context_line").Exists() && frame.Get("context_line").Type != gjson.Null
		hasPreContext := frame.Get("pre_context").Exists() && frame.Get("pre_context").IsArray()
		hasPostContext := frame.Get("post_context").Exists() && frame.Get("post_context").IsArray()
		isSomeLinesAvailable := hasContextLine || hasPreContext || hasPostContext

		if isSomeLinesAvailable && frame.Get("lineno").Exists() {
			sourceCode := make([]SourceCodeLine, 0)
			lineNo := int(frame.Get("lineno").Int())

			if hasPreContext {
				pre := frame.Get("pre_context").Array()
				for index, lineContent := range pre {
					sourceCode = append(sourceCode, SourceCodeLine{
						Line:    lineNo + index - len(pre),
						Content: lineContent.String(),
					})
				}
			}

			if hasContextLine {
				sourceCode = append(sourceCode, SourceCodeLine{
					Line:    lineNo,
					Content: frame.Get("context_line").String(),
				})
			}

			if hasPostContext {
				for index, lineContent := range frame.Get("post_context").Array() {
					sourceCode = append(sourceCode, SourceCodeLine{
						Line:    lineNo + index + 1,
						Content: lineContent.String(),
					})
				}
			}

			bt.SourceCode = sourceCode
		}

		if frame.Get("colno").Exists() && frame.Get("colno").Type != gjson.Null {
			col := int(frame.Get("colno").Int())
			bt.Column = &col
		}

		if fn := frame.Get("function"); fn.Exists() && fn.Type != gjson.Null && fn.String() != "" {
			bt.Function = fn.String()
		}

		if vars := frame.Get("vars"); vars.Exists() && vars.IsObject() {
			args := make([]string, 0)
			vars.ForEach(func(key, value gjson.Result) bool {
				args = append(args, flattenObject(value, key.String())...)
				return true
			})
			if len(args) > 0 {
				bt.Arguments = args
			}
		}

		backtrace = append(backtrace, bt)
	}

	if len(backtrace) == 0 {
		return nil
	}
	return backtrace
}

// ComposeContext returns Sentry contexts as raw JSON, or nil if absent.
func ComposeContext(eventPayload gjson.Result) json.RawMessage {
	contexts := eventPayload.Get("contexts")
	if !contexts.Exists() || contexts.Type == gjson.Null {
		return nil
	}
	return json.RawMessage(contexts.Raw)
}

// ComposeAddons copies selected Sentry fields into addons (same field list as Node).
func ComposeAddons(eventPayload gjson.Result) map[string]interface{} {
	addons := make(map[string]interface{})
	for _, field := range addonFields {
		value := eventPayload.Get(field)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(value.Raw), &decoded); err != nil {
			continue
		}
		addons[field] = decoded
	}
	if len(addons) == 0 {
		return nil
	}
	return addons
}

// ComposeUserData maps Sentry user to Hawk user.
func ComposeUserData(eventPayload gjson.Result) *HawkUser {
	user := eventPayload.Get("user")
	if !user.Exists() || user.Type == gjson.Null {
		return nil
	}

	id := "unknown"
	if user.Get("id").Exists() && user.Get("id").Type != gjson.Null {
		id = user.Get("id").String()
	}

	u := &HawkUser{ID: id}
	if username := user.Get("username"); username.Exists() && username.Type != gjson.Null {
		u.Name = username.String()
	}
	if email := user.Get("email"); email.Exists() && email.Type != gjson.Null {
		u.URL = email.String()
	}
	return u
}

// IsJsSDK reports whether the Sentry SDK name is a JavaScript-related SDK.
func IsJsSDK(eventPayload gjson.Result) bool {
	sdkName := eventPayload.Get("sdk.name")
	if !sdkName.Exists() || sdkName.Type == gjson.Null {
		return false
	}
	name := sdkName.String()
	if name == "" {
		return false
	}

	for _, jsSDK := range sentryJsSDK {
		if name == jsSDK || strings.Contains(name, jsSDK) {
			return true
		}
	}
	return false
}

// TransformToHawkFormat converts a Sentry event item into a Hawk broker message.
func TransformToHawkFormat(envelopeHeaders json.RawMessage, item EnvelopeItem, projectID string) (*HawkBrokerPayload, error) {
	if len(item.Payload) == 0 || string(item.Payload) == "null" {
		return nil, fmt.Errorf("Item payload is missing")
	}

	headers := gjson.ParseBytes(envelopeHeaders)
	eventPayload := gjson.ParseBytes(item.Payload)

	sentAt := headers.Get("sent_at").String()
	sentAtUnix, err := parseSentAtUnix(sentAt)
	if err != nil {
		return nil, err
	}

	isJs := IsJsSDK(eventPayload)
	catcherType := CatcherTypeDefault
	if isJs {
		catcherType = CatcherTypeJavaScript
	}

	title := ComposeTitle(eventPayload)
	level := eventPayload.Get("level").String()
	if level == "" {
		level = "error"
	}

	event := HawkEventPayload{
		Title:          title,
		Type:           level,
		CatcherVersion: CatcherVersion,
	}

	if bt := ComposeBacktrace(eventPayload); bt != nil {
		event.Backtrace = bt
	}
	if ctx := ComposeContext(eventPayload); ctx != nil {
		event.Context = ctx
	}
	if user := ComposeUserData(eventPayload); user != nil {
		event.User = user
	}
	if addons := ComposeAddons(eventPayload); addons != nil {
		event.Addons = map[string]interface{}{
			"sentry": addons,
		}
	}

	release := eventPayload.Get("release").String()
	if release == "" {
		release = headers.Get("trace.release").String()
	}
	if release != "" {
		event.Release = release
	}

	return &HawkBrokerPayload{
		ProjectID:   projectID,
		CatcherType: catcherType,
		Timestamp:   sentAtUnix,
		Payload:     event,
	}, nil
}

func parseSentAtUnix(sentAt string) (int64, error) {
	if sentAt == "" {
		return 0, fmt.Errorf("Invalid sent_at timestamp: %s", sentAt)
	}
	t, err := time.Parse(time.RFC3339Nano, sentAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, sentAt)
	}
	if err != nil {
		return 0, fmt.Errorf("Invalid sent_at timestamp: %s", sentAt)
	}
	return t.Unix(), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// flattenObject ports Node flattenObject — preserves key order via gjson.ForEach.
func flattenObject(obj gjson.Result, prefix string) []string {
	if obj.Type == gjson.Null {
		if prefix != "" {
			return []string{prefix + "=null"}
		}
		return []string{"null"}
	}

	if !obj.Exists() {
		if prefix != "" {
			return []string{prefix + "=undefined"}
		}
		return []string{"undefined"}
	}

	switch obj.Type {
	case gjson.JSON:
		if obj.IsArray() {
			arr := obj.Array()
			if len(arr) == 0 {
				if prefix != "" {
					return []string{prefix + "=[]"}
				}
				return []string{"[]"}
			}
			result := make([]string, 0)
			for index, value := range arr {
				key := strconv.Itoa(index)
				if prefix != "" {
					key = prefix + "." + key
				}
				result = append(result, flattenObject(value, key)...)
			}
			return result
		}
		if obj.IsObject() {
			entries := make([]gjson.Result, 0)
			keys := make([]string, 0)
			obj.ForEach(func(key, value gjson.Result) bool {
				keys = append(keys, key.String())
				entries = append(entries, value)
				return true
			})
			if len(keys) == 0 {
				if prefix != "" {
					return []string{prefix + "={}"}
				}
				return []string{"{}"}
			}
			result := make([]string, 0)
			for i, key := range keys {
				newPrefix := key
				if prefix != "" {
					newPrefix = prefix + "." + key
				}
				result = append(result, flattenObject(entries[i], newPrefix)...)
			}
			return result
		}
	}

	// Primitive
	val := formatPrimitive(obj)
	if prefix != "" {
		return []string{prefix + "=" + val}
	}
	return []string{val}
}

func formatPrimitive(obj gjson.Result) string {
	switch obj.Type {
	case gjson.True:
		return "true"
	case gjson.False:
		return "false"
	case gjson.Number:
		// Match JS number stringification for integers
		if obj.Num == float64(int64(obj.Num)) {
			return strconv.FormatInt(int64(obj.Num), 10)
		}
		return strconv.FormatFloat(obj.Num, 'f', -1, 64)
	case gjson.String:
		return obj.String()
	case gjson.Null:
		return "null"
	default:
		return obj.Raw
	}
}
