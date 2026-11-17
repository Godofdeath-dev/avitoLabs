package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	saved, checks, requests metric.Int64Counter
	duration                metric.Float64Histogram
	queue, workers          metric.Int64ObservableGauge
}

func NewPositionMetrics() (*PositionMetrics, error) {
	meter := otel.Meter(scopeName)
	m := &PositionMetrics{}
	var err error
	if m.saved, err = meter.Int64Counter("tripgo.positions.saved"); err != nil {
		return nil, fmt.Errorf("create saved positions metric: %w", err)
	}
	if m.checks, err = meter.Int64Counter("tripgo.stale_checks"); err != nil {
		return nil, fmt.Errorf("create stale checks metric: %w", err)
	}
	if m.requests, err = meter.Int64Counter("tripgo.position_requests"); err != nil {
		return nil, fmt.Errorf("create position requests metric: %w", err)
	}
	if m.duration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s")); err != nil {
		return nil, fmt.Errorf("create push duration metric: %w", err)
	}
	if m.queue, err = meter.Int64ObservableGauge("tripgo.position_requests.queue.size"); err != nil {
		return nil, fmt.Errorf("create queue metric: %w", err)
	}
	if m.workers, err = meter.Int64ObservableGauge("tripgo.position_requests.workers.active"); err != nil {
		return nil, fmt.Errorf("create workers metric: %w", err)
	}
	return m, nil
}

// BindPool читает размер очереди и занятость воркеров в момент сбора метрик.
func (m *PositionMetrics) BindPool(snapshot func() (int64, int64)) (func() error, error) {
	reg, err := otel.Meter(scopeName).RegisterCallback(func(_ context.Context, o metric.Observer) error {
		queued, busy := snapshot()
		o.ObserveInt64(m.queue, queued)
		o.ObserveInt64(m.workers, busy)
		return nil
	}, m.queue, m.workers)
	if err != nil {
		return nil, fmt.Errorf("register position pool metrics: %w", err)
	}
	return reg.Unregister, nil
}
func (m *PositionMetrics) PositionSaved(ctx context.Context) { m.saved.Add(ctx, 1) }
func (m *PositionMetrics) StaleCheck(ctx context.Context, result string) {
	m.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) PositionRequest(ctx context.Context, result string, seconds float64) {
	opts := metric.WithAttributes(attribute.String("result", result))
	m.requests.Add(ctx, 1, opts)
	// У отброшенной задачи HTTP-вызова не было, его длительность не учитываем.
	if result != "dropped" {
		m.duration.Record(ctx, seconds, opts)
	}
}
