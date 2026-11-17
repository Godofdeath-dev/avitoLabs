package push_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Godofdeath-dev/avitoLabs/internal/gateway/push"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

func TestPushContractAndStatuses(t *testing.T) {
	t.Parallel()
	for _, status := range []int{202, 400, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			trip := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/v1/push", r.URL.Path)
				require.Equal(t, http.MethodPost, r.Method)
				var body struct {
					RequestID   string            `json:"request_id"`
					RecipientID uuid.UUID         `json:"recipient_id"`
					Kind        string            `json:"kind"`
					Data        map[string]string `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.RequestID == "" || body.RecipientID != trip.DriverID || body.Kind != "REQUEST_POSITION" || body.Data["trip_id"] != trip.ID.String() {
					t.Errorf("incorrect push: %+v", body)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider().Tracer("test"))
			require.NoError(t, err)
			defer gateway.Close()
			err = gateway.RequestPosition(context.Background(), trip)
			if status == 202 {
				require.NoError(t, err)
			} else {
				var httpErr *push.HTTPError
				require.ErrorAs(t, err, &httpErr)
				require.Equal(t, status, httpErr.Status)
			}
		})
	}
}

func TestPushCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider().Tracer("test"))
	require.NoError(t, err)
	defer gateway.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- gateway.RequestPosition(ctx, model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("push did not start")
	}
	cancel()
	select {
	case err := <-done:
		require.True(t, errors.Is(err, context.Canceled))
		var status *push.HTTPError
		require.False(t, errors.As(err, &status))
	case <-time.After(3 * time.Second):
		t.Fatal("push ignored context")
	}
}

func TestPushTruncatedResponse(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusAccepted, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("short"))
			}))
			defer server.Close()
			gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider().Tracer("test"))
			require.NoError(t, err)
			defer gateway.Close()
			err = gateway.RequestPosition(context.Background(), model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()})
			if status == http.StatusAccepted {
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				var statusErr *push.HTTPError
				require.False(t, errors.As(err, &statusErr))
			} else {
				var statusErr *push.HTTPError
				require.ErrorAs(t, err, &statusErr)
				require.Equal(t, status, statusErr.Status)
			}
		})
	}
}
