package telemetry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
)

func TestPositionPoolMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(previous); _ = provider.Shutdown(context.Background()) })
	m, err := telemetry.NewPositionMetrics()
	require.NoError(t, err)
	unregister, err := m.BindPool(func() (int64, int64) { return 7, 3 })
	require.NoError(t, err)
	defer func() { require.NoError(t, unregister()) }()
	ctx := context.Background()
	m.PositionSaved(ctx)
	m.StaleCheck(ctx, "queued")
	m.PositionRequest(ctx, "success", 0.25)
	m.PositionRequest(ctx, "dropped", 0)
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	found := map[string]bool{}
	for _, scope := range data.ScopeMetrics {
		for _, metric := range scope.Metrics {
			found[metric.Name] = true
			switch metric.Name {
			case "tripgo.position_requests.queue.size":
				g, ok := metric.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				require.Equal(t, int64(7), g.DataPoints[0].Value)
			case "tripgo.position_requests.workers.active":
				g, ok := metric.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				require.Equal(t, int64(3), g.DataPoints[0].Value)
			case "tripgo.position_requests.duration":
				h, ok := metric.Data.(metricdata.Histogram[float64])
				require.True(t, ok)
				require.Len(t, h.DataPoints, 1)
				require.Equal(t, uint64(1), h.DataPoints[0].Count)
			}
		}
	}
	for _, name := range []string{"tripgo.positions.saved", "tripgo.stale_checks", "tripgo.position_requests.queue.size", "tripgo.position_requests.workers.active", "tripgo.position_requests", "tripgo.position_requests.duration"} {
		require.True(t, found[name], name)
	}
}
