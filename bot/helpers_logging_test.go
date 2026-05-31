package bot

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

	logCommand(context.Background(), nil, "error", "failed operation: %s", "boom")
	logCommand(context.Background(), nil, "success", "done")

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

func TestLogCommandIncludesActiveTraceIdentifiers(t *testing.T) {
	orig := slog.Default()
	handler := &recordingHandler{}
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(orig)

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	ctx, span := otel.Tracer("livid-bot/bot/test").Start(context.Background(), "test")
	defer span.End()

	logCommand(ctx, nil, "success", "done")

	if len(handler.records) != 1 {
		t.Fatalf("expected one log record, got %d", len(handler.records))
	}
	attrs := map[string]any{}
	handler.records[0].Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	if attrs["trace_id"] == nil || attrs["trace_id"] == "" {
		t.Fatalf("expected trace_id attr, got %+v", attrs)
	}
	if attrs["span_id"] == nil || attrs["span_id"] == "" {
		t.Fatalf("expected span_id attr, got %+v", attrs)
	}
}
