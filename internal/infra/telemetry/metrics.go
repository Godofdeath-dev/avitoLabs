package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const scopeName = "github.com/Godofdeath-dev/avitoLabs"

type Metrics struct {
	httpRequestDuration metric.Float64Histogram
	httpActiveRequests  metric.Int64UpDownCounter
	dbOperationDuration metric.Float64Histogram
	tripsCreated        metric.Int64Counter
	tripsCompleted      metric.Int64Counter
}

func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(scopeName)

	httpDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP server request duration in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create http.server.request.duration: %w", err)
	}

	httpActive, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of in-flight HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create http.server.active_requests: %w", err)
	}

	dbDuration, err := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Database client operation duration in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create db.client.operation.duration: %w", err)
	}

	tripsCreated, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Total number of created trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create tripgo.trips.created: %w", err)
	}

	tripsCompleted, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Total number of completed trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create tripgo.trips.completed: %w", err)
	}

	return &Metrics{
		httpRequestDuration: httpDuration,
		httpActiveRequests:  httpActive,
		dbOperationDuration: dbDuration,
		tripsCreated:        tripsCreated,
		tripsCompleted:      tripsCompleted,
	}, nil
}

func (m *Metrics) IncTripsCreated(ctx context.Context) {
	if m == nil {
		return
	}
	m.tripsCreated.Add(ctx, 1)
}

func (m *Metrics) IncTripsCompleted(ctx context.Context) {
	if m == nil {
		return
	}
	m.tripsCompleted.Add(ctx, 1)
}

func (m *Metrics) AddActiveRequests(ctx context.Context, delta int64, method, route string) {
	if m == nil {
		return
	}
	m.httpActiveRequests.Add(ctx, delta, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	))
}

func (m *Metrics) RecordHTTPDuration(ctx context.Context, seconds float64, method, route string, status int) {
	if m == nil {
		return
	}
	m.httpRequestDuration.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}

func (m *Metrics) RecordDBDuration(ctx context.Context, seconds float64, operation, collection, errType string) {
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("db.operation.name", operation),
		attribute.String("db.collection.name", collection),
	}
	if errType != "" {
		attrs = append(attrs, attribute.String("error.type", errType))
	}
	m.dbOperationDuration.Record(ctx, seconds, metric.WithAttributes(attrs...))
}
