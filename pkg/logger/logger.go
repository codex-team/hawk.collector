package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/codex-team/hawk.collector/pkg/otel"
)

var bg = context.Background()

func With(args ...any) *slog.Logger {
	return slog.Default().With(args...)
}

func SetupFromEnv(ctx context.Context) func(context.Context) error {
	level := parseSlogLevel(os.Getenv("LOG_LEVEL"))
	shutdown, err := otel.Setup(ctx, os.Getenv("OTEL_LOGS_ENDPOINT"), otel.Config{
		ServiceName:  os.Getenv("SERVICE_NAME"),
		Environment:  os.Getenv("DEPLOYMENT_ENVIRONMENT"),
		StdoutLevel:  level,
		StdoutWriter: os.Stdout,
	})
	slog.Info("✓ Log level set", "level", level.String())
	if err != nil {
		slog.Warn("failed to init OTLP logger", "error", err)
	}
	return shutdown
}

func Tracef(format string, args ...any) {
	if !slog.Default().Enabled(bg, slog.LevelDebug) {
		return
	}
	slog.Log(bg, slog.LevelDebug, fmt.Sprintf(format, args...), "trace", true)
}

func Debugf(format string, args ...any) {
	logf(slog.LevelDebug, format, args...)
}

func Infof(format string, args ...any) {
	logf(slog.LevelInfo, format, args...)
}

func Warnf(format string, args ...any) {
	logf(slog.LevelWarn, format, args...)
}

func Errorf(format string, args ...any) {
	logf(slog.LevelError, format, args...)
}

func Printf(format string, args ...any) {
	logf(slog.LevelInfo, format, args...)
}

func Println(args ...any) {
	logv(slog.LevelInfo, args...)
}

func Debug(args ...any) {
	logv(slog.LevelDebug, args...)
}

func Info(args ...any) {
	logv(slog.LevelInfo, args...)
}

func Warn(args ...any) {
	logv(slog.LevelWarn, args...)
}

func Error(args ...any) {
	logv(slog.LevelError, args...)
}

func Fatalf(format string, args ...any) {
	logf(slog.LevelError, format, args...)
	os.Exit(1)
}

func logf(level slog.Level, format string, args ...any) {
	if !slog.Default().Enabled(bg, level) {
		return
	}
	slog.Log(bg, level, fmt.Sprintf(format, args...))
}

func logv(level slog.Level, args ...any) {
	if !slog.Default().Enabled(bg, level) {
		return
	}
	slog.Log(bg, level, fmt.Sprint(args...))
}

func parseSlogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace", "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "fatal", "panic":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
