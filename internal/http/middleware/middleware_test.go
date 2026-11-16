package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/http/middleware"
)

func TestRequestSignals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		panics bool
	}{
		{"success", 201, false}, {"client error", 409, false}, {"server error", 500, false}, {"panic", 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := NewMockhttpMetrics(ctrl)
			const route = "/trips/{tripId}"
			active := int64(0)
			m.EXPECT().AddActiveRequests(gomock.Any(), int64(1), "GET", route).Do(func(_ context.Context, delta int64, _, _ string) { active += delta })
			m.EXPECT().AddActiveRequests(gomock.Any(), int64(-1), "GET", route).Do(func(_ context.Context, delta int64, _, _ string) { active += delta })
			m.EXPECT().RecordHTTPDuration(gomock.Any(), gomock.Any(), "GET", route, tc.status)
			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
			previous := otel.GetTracerProvider()
			otel.SetTracerProvider(provider)
			t.Cleanup(func() { otel.SetTracerProvider(previous); _ = provider.Shutdown(context.Background()) })
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			router := chi.NewRouter()
			router.Use(middleware.Recover(logger), middleware.Tracing("test"), middleware.Logging(logger), middleware.Metrics(m))
			router.Get(route, func(w http.ResponseWriter, r *http.Request) {
				if active != 1 {
					t.Errorf("active during handler = %d", active)
				}
				if chi.URLParam(r, "tripId") != "123" {
					t.Error("route parameter lost")
				}
				if tc.panics {
					panic("test panic")
				}
				w.WriteHeader(tc.status)
			})
			response := httptest.NewRecorder()
			middleware.ResolveRoute(router)(router).ServeHTTP(response, httptest.NewRequest("GET", "/trips/123", nil))
			if response.Code != tc.status {
				t.Fatalf("status %d, want %d", response.Code, tc.status)
			}
			if active != 0 {
				t.Errorf("active after request = %d", active)
			}
			dec := json.NewDecoder(&logs)
			var entry map[string]any
			if err := dec.Decode(&entry); err != nil {
				t.Fatal(err)
			}
			if entry["route"] != route || entry["status"] != float64(tc.status) {
				t.Errorf("incorrect request log: %v", entry)
			}
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("spans = %d", len(spans))
			}
			span := spans[0]
			if span.Name() != "GET "+route {
				t.Errorf("span name = %s", span.Name())
			}
			if tc.status >= 500 && span.Status().Code != codes.Error {
				t.Error("error span not marked")
			}
		})
	}
}
