package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestQuerySpanAttributes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous); _ = provider.Shutdown(context.Background()) })
	ctx, parent := provider.Tracer("test").Start(context.Background(), "HTTP request")
	tracer := newQueryTracer(nil)
	queryCtx := tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "INSERT INTO trip_status_history (trip_id) VALUES ($1)"})
	tracer.TraceQueryEnd(queryCtx, nil, pgx.TraceQueryEndData{Err: &pgconn.PgError{Code: "23514", Message: "check constraint"}})
	parent.End()
	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("spans=%d", len(spans))
	}
	attrs := map[string]string{}
	for _, a := range spans[0].Attributes() {
		attrs[string(a.Key)] = a.Value.AsString()
	}
	if attrs["db.collection.name"] != "trip_status_history" || attrs["db.operation.name"] != "INSERT" || attrs["error.type"] != "23514" {
		t.Fatalf("SQL attributes: %v", attrs)
	}
	if spans[0].Status().Code != codes.Error {
		t.Error("SQL error is not marked")
	}
	if spans[0].Parent().SpanID() != spans[1].SpanContext().SpanID() {
		t.Error("SQL span is not a child of HTTP span")
	}
	for _, a := range spans[1].Attributes() {
		if a.Key == "db.collection.name" {
			t.Error("database attribute leaked to HTTP span")
		}
	}
}
