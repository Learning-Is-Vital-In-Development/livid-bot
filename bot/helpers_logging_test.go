package bot

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(string) slog.Handler {
	return h
}

func TestLogCommandStructuredFieldsAndLevels(t *testing.T) {
	orig := slog.Default()
	handler := &recordingHandler{}
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(orig)

	logCommand(nil, "error", "failed operation: %s", "boom")
	logCommand(nil, "success", "done")

	if len(handler.records) != 2 {
		t.Fatalf("expected 2 log records, got %d", len(handler.records))
	}

	first := handler.records[0]
	if first.Level != slog.LevelError {
		t.Fatalf("expected first record level error, got %v", first.Level)
	}
	attrs := map[string]any{}
	first.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	for _, key := range []string{"cmd", "stage", "guild", "user"} {
		if _, ok := attrs[key]; !ok {
			t.Fatalf("missing structured field %q", key)
		}
	}

	second := handler.records[1]
	if second.Level != slog.LevelInfo {
		t.Fatalf("expected second record level info, got %v", second.Level)
	}
}
