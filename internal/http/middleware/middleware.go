//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
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

type httpMetrics interface {
	AddActiveRequests(ctx context.Context, delta int64, method, route string)
	RecordHTTPDuration(ctx context.Context, seconds float64, method, route string, status int)
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	if code >= 100 && code < 200 {
		r.ResponseWriter.WriteHeader(code)
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// ResolveRoute matches without executing a handler, so active-request labels are
// known before dispatch. The actual router still handles 404 and 405 responses.
type (
	routeKey        struct{}
	traceContextKey struct{}
	traceContext    struct{ ctx context.Context }
)

func ResolveRoute(router chi.Router) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := chi.NewRouteContext()
			if !router.Match(rc, r.Method, r.URL.Path) {
				rc = chi.NewRouteContext()
			}
			r = r.WithContext(context.WithValue(r.Context(), routeKey{}, rc.RoutePattern()))
			next.ServeHTTP(w, r)
		})
	}
}

func routePattern(r *http.Request) string {
	if pattern, ok := r.Context().Value(routeKey{}).(string); ok && pattern != "" {
		return pattern
	}
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
			state := &traceContext{ctx: r.Context()}
			r = r.WithContext(context.WithValue(r.Context(), traceContextKey{}, state))
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.ErrorContext(state.ctx, "panic recovered",
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
			if state, ok := r.Context().Value(traceContextKey{}).(*traceContext); ok {
				state.ctx = ctx
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				p := recover()
				if p != nil {
					rec.status = http.StatusInternalServerError
					defer func() { panic(p) }()
				}
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
				p := recover()
				if p != nil {
					rec.status = http.StatusInternalServerError
					defer func() { panic(p) }()
				}
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
func Metrics(m httpMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			route := routePattern(r)
			m.AddActiveRequests(r.Context(), 1, r.Method, route)
			defer func() {
				p := recover()
				if p != nil {
					rec.status = http.StatusInternalServerError
				}
				m.AddActiveRequests(r.Context(), -1, r.Method, route)
				m.RecordHTTPDuration(r.Context(), time.Since(start).Seconds(), r.Method, route, rec.status)
				if p != nil {
					panic(p)
				}
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
