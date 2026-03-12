package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	formatText = "text"
	formatJSON = "json"

	defaultLogFilePath       = ""
	defaultLogFileMaxSizeMB  = 10
	defaultLogFileMaxBackups = 900
	defaultLogFileMaxAgeDays = 730
	defaultLogFileCompress   = true
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

type Config struct {
	Format string
	Level  slog.Level
	File   FileConfig
}

type FileConfig struct {
	Enabled    bool
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

func Configure() (*slog.Logger, io.Closer, error) {
	cfg := configFromEnv(os.Getenv)
	output, closer, err := outputFromConfig(cfg, os.Stdout)
	if err != nil {
		return nil, nil, err
	}

	logger := New(cfg, output)
	slog.SetDefault(logger)
	return logger, closer, nil
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
		File: FileConfig{
			Enabled:    parseBool(getEnv("LOG_FILE_ENABLED"), false),
			Path:       parseString(getEnv("LOG_FILE_PATH"), defaultLogFilePath),
			MaxSizeMB:  parsePositiveInt(getEnv("LOG_FILE_MAX_SIZE_MB"), defaultLogFileMaxSizeMB),
			MaxBackups: parsePositiveInt(getEnv("LOG_FILE_MAX_BACKUPS"), defaultLogFileMaxBackups),
			MaxAgeDays: parsePositiveInt(getEnv("LOG_FILE_MAX_AGE_DAYS"), defaultLogFileMaxAgeDays),
			Compress:   parseBool(getEnv("LOG_FILE_COMPRESS"), defaultLogFileCompress),
		},
	}
}

func outputFromConfig(cfg Config, out io.Writer) (io.Writer, io.Closer, error) {
	if !cfg.File.Enabled {
		return out, nopCloser{}, nil
	}

	path := strings.TrimSpace(cfg.File.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("LOG_FILE_ENABLED=true requires LOG_FILE_PATH")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log directory %s: %w", dir, err)
	}

	fileSink := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.File.MaxSizeMB,
		MaxBackups: cfg.File.MaxBackups,
		MaxAge:     cfg.File.MaxAgeDays,
		Compress:   cfg.File.Compress,
	}
	return io.MultiWriter(out, fileSink), fileSink, nil
}

func parseFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case formatText:
		return formatText
	case formatJSON:
		return formatJSON
	default:
		return formatJSON
	}
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parsePositiveInt(value string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
