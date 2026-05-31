package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace/noop"
)

func resetForTest() {
	configureMu.Lock()
	defer configureMu.Unlock()
	configured = false
	shutdown = noopShutdown
	otel.SetTracerProvider(noop.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
}
