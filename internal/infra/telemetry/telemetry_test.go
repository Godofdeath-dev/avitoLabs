package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
)

func TestLoggerCorrelatesDerivedLogger(t *testing.T) {
	var output bytes.Buffer
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	defer span.End()
	logger := telemetry.NewLogger("info", "trip-service", &output).With("component", "test")
	logger.DebugContext(ctx, "hidden")
	logger.InfoContext(ctx, "trip created")
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["trace_id"] != span.SpanContext().TraceID().String() || entry["span_id"] != span.SpanContext().SpanID().String() {
		t.Fatalf("missing correlation: %v", entry)
	}
	if entry["service"] != "trip-service" || entry["component"] != "test" {
		t.Fatalf("missing fields: %v", entry)
	}
}
