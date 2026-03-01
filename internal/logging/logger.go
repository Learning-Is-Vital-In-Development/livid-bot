package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	formatText = "text"
	formatJSON = "json"
)

type Config struct {
	Format string
	Level  slog.Level
}

func Configure() *slog.Logger {
	cfg := configFromEnv(os.Getenv)
	logger := New(cfg, os.Stdout)
	slog.SetDefault(logger)
	return logger
}

func New(cfg Config, output io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Level}

	switch cfg.Format {
	case formatJSON:
		return slog.New(slog.NewJSONHandler(output, opts))
	default:
		return slog.New(slog.NewTextHandler(output, opts))
	}
}

func configFromEnv(getEnv func(string) string) Config {
	return Config{
		Format: parseFormat(getEnv("LOG_FORMAT")),
		Level:  parseLevel(getEnv("LOG_LEVEL")),
	}
}

func parseFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case formatJSON:
		return formatJSON
	default:
		return formatText
	}
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
