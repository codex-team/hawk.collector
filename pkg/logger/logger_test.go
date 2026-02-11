package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestWithAddsAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	With("projectId", "proj-123").Info("hello")

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal log: %v", err)
	}

	if got := payload["projectId"]; got != "proj-123" {
		t.Fatalf("expected projectId=proj-123, got %v", got)
	}
	if got := payload["msg"]; got != "hello" {
		t.Fatalf("expected msg=hello, got %v", got)
	}
}
