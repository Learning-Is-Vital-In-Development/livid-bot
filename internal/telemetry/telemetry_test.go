package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestConfigFromEnvDefaultsToDisabledLividBotService(t *testing.T) {
	cfg := configFromEnv(func(string) string { return "" })

	if cfg.Enabled() {
		t.Fatal("expected tracing to be disabled without OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if cfg.ServiceName != "livid-bot" {
		t.Fatalf("expected default service name livid-bot, got %q", cfg.ServiceName)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("expected default app env development, got %q", cfg.AppEnv)
	}
	if cfg.Protocol != "http/protobuf" {
		t.Fatalf("expected default OTLP protocol http/protobuf, got %q", cfg.Protocol)
	}
	if cfg.Sampler != "parentbased_traceidratio" {
		t.Fatalf("expected default sampler parentbased_traceidratio, got %q", cfg.Sampler)
	}
	if cfg.SamplerArg != "1.0" {
		t.Fatalf("expected default sampler arg 1.0, got %q", cfg.SamplerArg)
	}
}

func TestResourceAttributesMergeServiceEnvironmentAndEnvAttributes(t *testing.T) {
	env := map[string]string{
		"APP_ENV":                  "production",
		"OTEL_SERVICE_NAME":        "custom-livid",
		"OTEL_RESOURCE_ATTRIBUTES": "service.namespace=livid-bot,compose.project=livid,empty=,malformed",
	}
	cfg := configFromEnv(func(key string) string { return env[key] })

	attrs := cfg.ResourceAttributes()

	want := map[string]string{
		"service.name":           "custom-livid",
		"deployment.environment": "production",
		"service.namespace":      "livid-bot",
		"compose.project":        "livid",
	}
	for key, value := range want {
		if attrs[key] != value {
			t.Fatalf("expected resource attribute %s=%q, got %q", key, value, attrs[key])
		}
	}
	if _, ok := attrs["empty"]; ok {
		t.Fatal("expected empty resource attribute values to be ignored")
	}
	if _, ok := attrs["malformed"]; ok {
		t.Fatal("expected malformed resource attributes to be ignored")
	}
}

func TestConfigureNoopsWithoutEndpoint(t *testing.T) {
	resetForTest()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Configure(context.Background())
	if err != nil {
		t.Fatalf("Configure returned error without endpoint: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected no-op shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestTraceIDFromContextReturnsCurrentSpanIdentifiers(t *testing.T) {
	resetForTest()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		resetForTest()
	})

	ctx, span := otel.Tracer("livid-bot/internal/telemetry/test").Start(context.Background(), "test-span")
	defer span.End()

	traceID, spanID := TraceIDsFromContext(ctx)
	if traceID == "" {
		t.Fatal("expected trace id from active span")
	}
	if spanID == "" {
		t.Fatal("expected span id from active span")
	}
}
