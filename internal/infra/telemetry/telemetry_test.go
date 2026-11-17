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

func TestLoggerWithoutSpanHasRequiredFields(t *testing.T) {
	var output bytes.Buffer
	logger := telemetry.NewLogger("info", "trip-service", &output)
	logger.InfoContext(context.Background(), "positions worker started")
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"time", "level", "msg", "service", "trace_id", "span_id"} {
		if _, exists := entry[field]; !exists {
			t.Errorf("missing field %s", field)
		}
	}
	if entry["trace_id"] != "" || entry["span_id"] != "" {
		t.Fatalf("unexpected trace context: %v", entry)
	}
}
