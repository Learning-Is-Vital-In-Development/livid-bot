package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	cfg := configFromEnv(func(string) string { return "" })

	if cfg.Format != formatText {
		t.Fatalf("expected default format %q, got %q", formatText, cfg.Format)
	}
	if cfg.Level != slog.LevelInfo {
		t.Fatalf("expected default level info, got %v", cfg.Level)
	}
}

func TestConfigFromEnvCustom(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		switch key {
		case "LOG_FORMAT":
			return "json"
		case "LOG_LEVEL":
			return "debug"
		default:
			return ""
		}
	})

	if cfg.Format != formatJSON {
		t.Fatalf("expected format %q, got %q", formatJSON, cfg.Format)
	}
	if cfg.Level != slog.LevelDebug {
		t.Fatalf("expected level debug, got %v", cfg.Level)
	}
}

func TestConfigFromEnvInvalidFallback(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		switch key {
		case "LOG_FORMAT":
			return "xml"
		case "LOG_LEVEL":
			return "trace"
		default:
			return ""
		}
	})

	if cfg.Format != formatText {
		t.Fatalf("expected invalid format fallback to %q, got %q", formatText, cfg.Format)
	}
	if cfg.Level != slog.LevelInfo {
		t.Fatalf("expected invalid level fallback to info, got %v", cfg.Level)
	}
}

func TestNewLoggerFormatAndLevel(t *testing.T) {
	logger := New(Config{Format: formatJSON, Level: slog.LevelWarn}, io.Discard)

	if got := fmt.Sprintf("%T", logger.Handler()); got != "*slog.JSONHandler" {
		t.Fatalf("expected JSON handler, got %s", got)
	}
	if logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatalf("expected info level to be disabled for warn logger")
	}
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatalf("expected warn level to be enabled")
	}
}
