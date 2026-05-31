package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultServiceName = "livid-bot"
	defaultAppEnv      = "development"
	defaultProtocol    = "http/protobuf"
	defaultSampler     = "parentbased_traceidratio"
	defaultSamplerArg  = "1.0"
)

var (
	configureMu sync.Mutex
	configured  bool
	shutdown    = noopShutdown
)

type Config struct {
	Endpoint              string
	Protocol              string
	ServiceName           string
	AppEnv                string
	RawResourceAttributes string
	Sampler               string
	SamplerArg            string
}

func (c Config) Enabled() bool {
	return strings.TrimSpace(c.Endpoint) != ""
}

func (c Config) ResourceAttributes() map[string]string {
	attributes := map[string]string{
		"service.name":           c.ServiceName,
		"deployment.environment": c.AppEnv,
	}

	for _, item := range strings.Split(c.RawResourceAttributes, ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		attributes[key] = value
	}

	return attributes
}

func Configure(ctx context.Context) (func(context.Context) error, error) {
	cfg := configFromEnv(os.Getenv)
	if !cfg.Enabled() {
		return noopShutdown, nil
	}
	if cfg.Protocol != defaultProtocol {
		return nil, fmt.Errorf("unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q: only %q is supported", cfg.Protocol, defaultProtocol)
	}

	configureMu.Lock()
	defer configureMu.Unlock()

	if configured {
		return shutdown, nil
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes("", resourceAttributes(cfg)...)),
		sdktrace.WithSampler(sampler(cfg)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	shutdown = provider.Shutdown
	configured = true
	return shutdown, nil
}

func TraceIDsFromContext(ctx context.Context) (traceID, spanID string) {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return "", ""
	}
	return spanContext.TraceID().String(), spanContext.SpanID().String()
}

func configFromEnv(getenv func(string) string) Config {
	appEnv := firstNonEmpty(getenv("APP_ENV"), defaultAppEnv)
	return Config{
		Endpoint:              strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		Protocol:              firstNonEmpty(getenv("OTEL_EXPORTER_OTLP_PROTOCOL"), defaultProtocol),
		ServiceName:           firstNonEmpty(getenv("OTEL_SERVICE_NAME"), defaultServiceName),
		AppEnv:                appEnv,
		RawResourceAttributes: getenv("OTEL_RESOURCE_ATTRIBUTES"),
		Sampler:               firstNonEmpty(getenv("OTEL_TRACES_SAMPLER"), defaultSampler),
		SamplerArg:            firstNonEmpty(getenv("OTEL_TRACES_SAMPLER_ARG"), defaultSamplerArg),
	}
}

func resourceAttributes(cfg Config) []attribute.KeyValue {
	attrs := cfg.ResourceAttributes()
	keys := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		keys = append(keys, attribute.String(key, value))
	}
	return keys
}

func sampler(cfg Config) sdktrace.Sampler {
	if cfg.Sampler != defaultSampler {
		return sdktrace.AlwaysSample()
	}

	ratio, err := strconv.ParseFloat(strings.TrimSpace(cfg.SamplerArg), 64)
	if err != nil {
		ratio = 1.0
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
}

func firstNonEmpty(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func noopShutdown(context.Context) error { return nil }
