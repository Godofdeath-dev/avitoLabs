package config_test

import (
	"testing"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
)

func TestConfigRejectsInvalidTimeoutsAndPool(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_QUERY_TIMEOUT", "0s"}, {"SHUTDOWN_TIMEOUT", "-1s"}, {"HTTP_READ_TIMEOUT", "invalid"}, {"DATABASE_MAX_CONNS", "0"}, {"DATABASE_MIN_CONNS", "-1"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", ":8080")
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv(tc.key, tc.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
