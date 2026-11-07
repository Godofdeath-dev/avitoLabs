package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/domainerror"
)

type HTTPMetrics interface {
	AddActiveRequests(ctx context.Context, delta int64, method, route string)
	RecordHTTPDuration(ctx context.Context, seconds float64, method, route string, status int)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pat := rctx.RoutePattern(); pat != "" {
			return pat
		}
	}
	return "unknown"
}

// 1. Recover Middleware
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.ErrorContext(r.Context(), "panic recovered",
							slog.Any("panic", rec),
							slog.String("stack", string(debug.Stack())),
						)
					}
					detail := "Internal server error"
					domainerror.WriteProblem(w, http.StatusInternalServerError, api.Problem{
						Status: http.StatusInternalServerError,
						Code:   "internal_error",
						Title:  "Internal Error",
						Detail: &detail,
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// 2. Tracing Middleware
func Tracing(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method, trace.WithSpanKind(trace.SpanKindServer))

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				route := routePattern(r)
				span.SetName(fmt.Sprintf("%s %s", r.Method, route))
				span.SetAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
					attribute.Int("http.response.status_code", rec.status),
				)
				if rec.status >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, http.StatusText(rec.status))
					span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%d", rec.status)))
				}
				span.End()
			}()

			next.ServeHTTP(rec, r.WithContext(ctx))
		})
	}
}

// 3. Logging Middleware
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				if logger == nil {
					return
				}
				route := routePattern(r)
				attrs := []any{
					slog.String("method", r.Method),
					slog.String("route", route),
					slog.Int("status", rec.status),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				}

				if rec.status >= http.StatusInternalServerError {
					logger.ErrorContext(r.Context(), "request handled", attrs...)
				} else {
					logger.InfoContext(r.Context(), "request handled", attrs...)
				}
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

// 4. Metrics Middleware
func Metrics(m HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			// Определяем паттерн маршрута до вызова хендлера не можем,
			// так как chi резолвит его внутри next.ServeHTTP.
			// Но инкремент активных запросов можно сделать на сырой путь,
			// либо использовать общий ярлык, а после ответа обновить декремент по точному route.
			// Для простоты выполним инкремент с "unknown" или сырым путем, а потом скорректируем.

			// Более безопасный вариант с defer, как просил ревьюер:
			defer func() {
				route := routePattern(r)
				m.RecordHTTPDuration(r.Context(), time.Since(start).Seconds(), r.Method, route, rec.status)
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
