package otel

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	logapi "go.opentelemetry.io/otel/log"
)

type captureLogger struct {
	logapi.Logger
	mu      sync.Mutex
	records []logapi.Record
}

func (c *captureLogger) logger() {}

func (c *captureLogger) Emit(_ context.Context, record logapi.Record) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, record)
}

func (c *captureLogger) Enabled(context.Context, logapi.EnabledParameters) bool {
	return true
}

func (c *captureLogger) lastRecord() (logapi.Record, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.records) == 0 {
		return logapi.Record{}, false
	}
	return c.records[len(c.records)-1], true
}

func TestOtelSlogHandlerSeverity(t *testing.T) {
	logger := &captureLogger{}
	handler := otelSlogHandler{logger: logger}

	record := slog.NewRecord(time.Now(), slog.LevelWarn, "warn message", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	got, ok := logger.lastRecord()
	if !ok {
		t.Fatal("expected record, got none")
	}
	if got.Severity() != logapi.SeverityWarn {
		t.Fatalf("expected severity %v, got %v", logapi.SeverityWarn, got.Severity())
	}
}

func TestOtelSlogHandlerAttributes(t *testing.T) {
	logger := &captureLogger{}
	handler := otelSlogHandler{logger: logger}

	handler = handler.WithGroup("ctx").WithAttrs([]slog.Attr{slog.String("foo", "bar")}).(otelSlogHandler)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)
	record.AddAttrs(slog.Int("count", 2))

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	got, ok := logger.lastRecord()
	if !ok {
		t.Fatal("expected record, got none")
	}

	attrs := map[string]logapi.Value{}
	got.WalkAttributes(func(kv logapi.KeyValue) bool {
		attrs[kv.Key] = kv.Value
		return true
	})

	if _, ok := attrs["ctx.foo"]; !ok {
		t.Fatalf("expected attribute ctx.foo")
	}
	if _, ok := attrs["ctx.count"]; !ok {
		t.Fatalf("expected attribute ctx.count")
	}
}

func TestOtelSlogHandlerAnyValue(t *testing.T) {
	logger := &captureLogger{}
	handler := otelSlogHandler{logger: logger}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)
	record.AddAttrs(slog.Any("meta", map[string]int{"a": 1}))

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	got, ok := logger.lastRecord()
	if !ok {
		t.Fatal("expected record, got none")
	}

	found := false
	got.WalkAttributes(func(kv logapi.KeyValue) bool {
		if kv.Key == "meta" {
			found = kv.Value.Kind() == logapi.KindString
			return false
		}
		return true
	})

	if !found {
		t.Fatalf("expected meta attribute as string")
	}
}
