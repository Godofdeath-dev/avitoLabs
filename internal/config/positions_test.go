package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
)

func TestPositionsConfiguration(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("PUSH_HTTP_URL", "http://localhost:8090")
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, cfg.PositionCheckInterval)
	require.Equal(t, 60*time.Second, cfg.PositionStaleAfter)
	require.Equal(t, 4, cfg.PositionWorkers)
	require.Equal(t, 100, cfg.PositionQueueSize)
	for _, tc := range []struct{ key, value string }{{"POSITION_WORKERS", "0"}, {"POSITION_QUEUE_SIZE", "-1"}, {"POSITION_STALE_AFTER", "0s"}, {"PUSH_REQUEST_TIMEOUT", "bad"}, {"PUSH_HTTP_URL", ""}} {
		t.Run(tc.key, func(t *testing.T) { t.Setenv(tc.key, tc.value); _, err := config.Load(); require.Error(t, err) })
	}
}
