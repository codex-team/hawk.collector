package otel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	logapi "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Config struct {
	ServiceName  string
	Environment  string
	StdoutJSON   bool
	StdoutLevel  slog.Leveler
	StdoutWriter io.Writer
}

func Setup(ctx context.Context, endpoint string, cfg Config) (func(context.Context) error, error) {
	writer := cfg.StdoutWriter
	if writer == nil {
		writer = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{Level: cfg.StdoutLevel}
	var stdoutHandler slog.Handler
	if cfg.StdoutJSON {
		stdoutHandler = slog.NewJSONHandler(writer, handlerOpts)
	} else {
		stdoutHandler = slog.NewTextHandler(writer, handlerOpts)
	}

	handlers := []slog.Handler{stdoutHandler}
	shutdown := func(context.Context) error { return nil }

	if endpoint == "" {
		slog.SetDefault(slog.New(teeHandler{handlers: handlers}))
		return shutdown, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("http://" + endpoint)
		if err != nil {
			slog.SetDefault(slog.New(teeHandler{handlers: handlers}))
			return shutdown, err
		}
	}

	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(parsed.Host),
	}
	if parsed.Scheme == "" || parsed.Scheme == "http" {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	if path := strings.TrimSpace(parsed.EscapedPath()); path != "" {
		opts = append(opts, otlploghttp.WithURLPath(path))
	}

	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		slog.SetDefault(slog.New(teeHandler{handlers: handlers}))
		return shutdown, err
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "collector"
	}

	leveler := cfg.StdoutLevel
	if leveler == nil {
		leveler = slog.LevelInfo
	}

	processor := sdklog.NewBatchProcessor(exporter)
	attrs := []attribute.KeyValue{
		attribute.String("service.name", serviceName),
	}
	if env := strings.TrimSpace(cfg.Environment); env != "" {
		attrs = append(attrs, attribute.String("deployment.environment", env))
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(processor),
		sdklog.WithResource(resource.NewWithAttributes("", attrs...)),
	)

	logglobal.SetLoggerProvider(provider)
	otelHandler := otelSlogHandler{
		logger:  logglobal.Logger(serviceName),
		leveler: leveler,
	}
	handlers = append(handlers, otelHandler)
	slog.SetDefault(slog.New(teeHandler{handlers: handlers}))

	shutdown = provider.Shutdown
	return shutdown, nil
}

type teeHandler struct {
	handlers []slog.Handler
}

func (t teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range t.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, handler := range t.handlers {
		if handler.Enabled(ctx, record.Level) {
			if handleErr := handler.Handle(ctx, record); handleErr != nil {
				err = errors.Join(err, handleErr)
			}
		}
	}
	return err
}

func (t teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(t.handlers))
	for _, handler := range t.handlers {
		next = append(next, handler.WithAttrs(attrs))
	}
	return teeHandler{handlers: next}
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(t.handlers))
	for _, handler := range t.handlers {
		next = append(next, handler.WithGroup(name))
	}
	return teeHandler{handlers: next}
}

type otelSlogHandler struct {
	logger  logapi.Logger
	leveler slog.Leveler
	attrs   []slog.Attr
	groups  []string
}

func (h otelSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.leveler == nil {
		return level >= slog.LevelInfo
	}
	return level >= h.leveler.Level()
}

func (h otelSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.logger == nil {
		return nil
	}

	var otelRecord logapi.Record
	if record.Time.IsZero() {
		otelRecord.SetTimestamp(time.Now())
	} else {
		otelRecord.SetTimestamp(record.Time)
	}
	otelRecord.SetObservedTimestamp(time.Now())
	otelRecord.SetSeverity(otelSeverity(record.Level))
	otelRecord.SetSeverityText(record.Level.String())
	otelRecord.SetBody(logapi.StringValue(record.Message))

	prefix := strings.Join(h.groups, ".")
	for _, attr := range h.attrs {
		appendAttr(&otelRecord, prefix, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(&otelRecord, prefix, attr)
		return true
	})

	h.logger.Emit(ctx, otelRecord)
	return nil
}

func (h otelSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := otelSlogHandler{
		logger:  h.logger,
		leveler: h.leveler,
		attrs:   append(append([]slog.Attr(nil), h.attrs...), attrs...),
		groups:  append([]string(nil), h.groups...),
	}
	return next
}

func (h otelSlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := otelSlogHandler{
		logger:  h.logger,
		leveler: h.leveler,
		attrs:   append([]slog.Attr(nil), h.attrs...),
		groups:  append(append([]string(nil), h.groups...), name),
	}
	return next
}

func appendAttr(record *logapi.Record, prefix string, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}

	key := joinKey(prefix, attr.Key)

	switch attr.Value.Kind() {
	case slog.KindGroup:
		groupPrefix := key
		for _, groupAttr := range attr.Value.Group() {
			appendAttr(record, groupPrefix, groupAttr)
		}
	default:
		record.AddAttributes(logapi.KeyValue{
			Key:   key,
			Value: otelValueFromSlog(attr.Value),
		})
	}
}

func joinKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

func otelSeverity(level slog.Level) logapi.Severity {
	switch {
	case level >= slog.LevelError:
		return logapi.SeverityError
	case level >= slog.LevelWarn:
		return logapi.SeverityWarn
	case level >= slog.LevelInfo:
		return logapi.SeverityInfo
	default:
		return logapi.SeverityDebug
	}
}

func otelValueFromSlog(value slog.Value) logapi.Value {
	switch value.Kind() {
	case slog.KindString:
		return logapi.StringValue(value.String())
	case slog.KindInt64:
		return logapi.Int64Value(value.Int64())
	case slog.KindUint64:
		if value.Uint64() <= math.MaxInt64 {
			return logapi.Int64Value(int64(value.Uint64()))
		}
		return logapi.StringValue(value.String())
	case slog.KindFloat64:
		return logapi.Float64Value(value.Float64())
	case slog.KindBool:
		return logapi.BoolValue(value.Bool())
	case slog.KindDuration:
		return logapi.StringValue(value.Duration().String())
	case slog.KindTime:
		return logapi.StringValue(value.Time().Format(time.RFC3339Nano))
	case slog.KindAny:
		return logapi.StringValue(fmt.Sprint(value.Any()))
	default:
		return logapi.StringValue(value.String())
	}
}
