package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	cfg := configFromEnv(func(string) string { return "" })

	if cfg.Format != formatJSON {
		t.Fatalf("expected default format %q, got %q", formatJSON, cfg.Format)
	}
	if cfg.Level != slog.LevelInfo {
		t.Fatalf("expected default level info, got %v", cfg.Level)
	}
	if cfg.File.Enabled {
		t.Fatalf("expected file logging to be disabled by default")
	}
	if cfg.File.Path != "" {
		t.Fatalf("expected default log file path to be empty, got %q", cfg.File.Path)
	}
	if cfg.File.MaxSizeMB != defaultLogFileMaxSizeMB {
		t.Fatalf("expected default log file max size %d, got %d", defaultLogFileMaxSizeMB, cfg.File.MaxSizeMB)
	}
	if cfg.File.MaxBackups != defaultLogFileMaxBackups {
		t.Fatalf("expected default max backups %d, got %d", defaultLogFileMaxBackups, cfg.File.MaxBackups)
	}
	if cfg.File.MaxAgeDays != defaultLogFileMaxAgeDays {
		t.Fatalf("expected default max age %d, got %d", defaultLogFileMaxAgeDays, cfg.File.MaxAgeDays)
	}
	if cfg.File.Compress != defaultLogFileCompress {
		t.Fatalf("expected default compress=%t, got %t", defaultLogFileCompress, cfg.File.Compress)
	}
}

func TestConfigFromEnvCustom(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		switch key {
		case "LOG_FORMAT":
			return "json"
		case "LOG_LEVEL":
			return "debug"
		case "LOG_FILE_ENABLED":
			return "true"
		case "LOG_FILE_PATH":
			return "/tmp/custom.log"
		case "LOG_FILE_MAX_SIZE_MB":
			return "15"
		case "LOG_FILE_MAX_BACKUPS":
			return "20"
		case "LOG_FILE_MAX_AGE_DAYS":
			return "365"
		case "LOG_FILE_COMPRESS":
			return "false"
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
	if !cfg.File.Enabled {
		t.Fatalf("expected file logging to be enabled")
	}
	if cfg.File.Path != "/tmp/custom.log" {
		t.Fatalf("expected custom log file path, got %q", cfg.File.Path)
	}
	if cfg.File.MaxSizeMB != 15 {
		t.Fatalf("expected max size 15, got %d", cfg.File.MaxSizeMB)
	}
	if cfg.File.MaxBackups != 20 {
		t.Fatalf("expected max backups 20, got %d", cfg.File.MaxBackups)
	}
	if cfg.File.MaxAgeDays != 365 {
		t.Fatalf("expected max age 365, got %d", cfg.File.MaxAgeDays)
	}
	if cfg.File.Compress {
		t.Fatalf("expected compress=false")
	}
}

func TestConfigFromEnvExplicitTextFormat(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		if key == "LOG_FORMAT" {
			return "text"
		}
		return ""
	})

	if cfg.Format != formatText {
		t.Fatalf("expected explicit format %q, got %q", formatText, cfg.Format)
	}
}

func TestConfigFromEnvInvalidFallback(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		switch key {
		case "LOG_FORMAT":
			return "xml"
		case "LOG_LEVEL":
			return "trace"
		case "LOG_FILE_ENABLED":
			return "nope"
		case "LOG_FILE_PATH":
			return "   "
		case "LOG_FILE_MAX_SIZE_MB":
			return "0"
		case "LOG_FILE_MAX_BACKUPS":
			return "-1"
		case "LOG_FILE_MAX_AGE_DAYS":
			return "NaN"
		case "LOG_FILE_COMPRESS":
			return "not-bool"
		default:
			return ""
		}
	})

	if cfg.Format != formatJSON {
		t.Fatalf("expected invalid format fallback to %q, got %q", formatJSON, cfg.Format)
	}
	if cfg.Level != slog.LevelInfo {
		t.Fatalf("expected invalid level fallback to info, got %v", cfg.Level)
	}
	if cfg.File.Enabled {
		t.Fatalf("expected invalid bool to fallback to disabled")
	}
	if cfg.File.Path != "" {
		t.Fatalf("expected blank file path to fallback to empty, got %q", cfg.File.Path)
	}
	if cfg.File.MaxSizeMB != defaultLogFileMaxSizeMB {
		t.Fatalf("expected invalid max size fallback to %d, got %d", defaultLogFileMaxSizeMB, cfg.File.MaxSizeMB)
	}
	if cfg.File.MaxBackups != defaultLogFileMaxBackups {
		t.Fatalf("expected invalid max backups fallback to %d, got %d", defaultLogFileMaxBackups, cfg.File.MaxBackups)
	}
	if cfg.File.MaxAgeDays != defaultLogFileMaxAgeDays {
		t.Fatalf("expected invalid max age fallback to %d, got %d", defaultLogFileMaxAgeDays, cfg.File.MaxAgeDays)
	}
	if cfg.File.Compress != defaultLogFileCompress {
		t.Fatalf("expected invalid compress fallback to %t, got %t", defaultLogFileCompress, cfg.File.Compress)
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

func TestOutputFromConfigDisabled(t *testing.T) {
	out := &bytes.Buffer{}
	writer, closer, err := outputFromConfig(Config{File: FileConfig{Enabled: false}}, out)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer != out {
		t.Fatalf("expected disabled file logging to return original writer")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("expected nopCloser.Close() to return nil, got %v", err)
	}
}

func TestOutputFromConfigEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bot.log")
	stdout := &bytes.Buffer{}

	cfg := Config{
		File: FileConfig{
			Enabled:    true,
			Path:       path,
			MaxSizeMB:  10,
			MaxBackups: 2,
			MaxAgeDays: 1,
			Compress:   false,
		},
	}

	writer, closer, err := outputFromConfig(cfg, stdout)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		if err := closer.Close(); err != nil {
			t.Fatalf("expected Close() to return nil, got %v", err)
		}
	}()

	if _, err := writer.Write([]byte("hello world\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if got := stdout.String(); got != "hello world\n" {
		t.Fatalf("expected stdout writer to receive output, got %q", got)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected log file to be created: %v", err)
	}
	if !strings.Contains(string(raw), "hello world") {
		t.Fatalf("expected log file to contain output, got %q", string(raw))
	}
}

func TestOutputFromConfigEnabledRequiresPath(t *testing.T) {
	_, _, err := outputFromConfig(
		Config{File: FileConfig{Enabled: true, Path: "   "}},
		io.Discard,
	)
	if err == nil {
		t.Fatalf("expected error when file logging is enabled without path")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{"warning", slog.LevelInfo}, // non-standard alias removed
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewLoggerTextFormat(t *testing.T) {
	logger := New(Config{Format: formatText, Level: slog.LevelInfo}, io.Discard)

	if got := fmt.Sprintf("%T", logger.Handler()); got != "*slog.TextHandler" {
		t.Fatalf("expected TextHandler, got %s", got)
	}
}

func TestOutputFromConfigMkdirAllError(t *testing.T) {
	cfg := Config{
		File: FileConfig{
			Enabled: true,
			Path:    "/dev/null/impossible/bot.log",
		},
	}
	_, _, err := outputFromConfig(cfg, io.Discard)
	if err == nil {
		t.Fatalf("expected MkdirAll error for invalid directory")
	}
	if !strings.Contains(err.Error(), "create log directory") {
		t.Fatalf("expected error to mention directory creation, got: %v", err)
	}
}

func TestConfigure(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FILE_ENABLED", "false")

	logger, closer, err := Configure()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}
	if closer == nil {
		t.Fatalf("expected non-nil closer")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("expected Close() to return nil, got %v", err)
	}
	if got := fmt.Sprintf("%T", logger.Handler()); got != "*slog.JSONHandler" {
		t.Fatalf("expected JSONHandler, got %s", got)
	}
	if logger.Enabled(context.Background(), slog.LevelDebug) != true {
		t.Fatalf("expected debug level to be enabled")
	}
}
