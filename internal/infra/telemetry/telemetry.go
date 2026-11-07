package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type traceLogHandler struct {
	slog.Handler
}

func (h *traceLogHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceLogHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceLogHandler) WithGroup(name string) slog.Handler {
	return &traceLogHandler{Handler: h.Handler.WithGroup(name)}
}

func NewLogger(levelStr, serviceName string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	baseHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
	})
	return slog.New(&traceLogHandler{Handler: baseHandler})
}

func SetupOTel(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		return err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return shutdown, err
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Если переменная OTEL_EXPORTER_OTLP_ENDPOINT не задана, работаем с no-op без падения
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return shutdown, nil
	}

	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(2*time.Second))
	if err != nil {
		return shutdown, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(time.Second)),
	)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second))
	if err != nil {
		return shutdown, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	return shutdown, nil
}
