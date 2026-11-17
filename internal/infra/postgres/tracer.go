package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type dbMetrics interface {
	RecordDBDuration(context.Context, float64, string, string, string)
}

type (
	queryKey  struct{}
	queryInfo struct {
		started               time.Time
		operation, collection string
	}
)

type queryTracer struct {
	*otelpgx.Tracer
	metrics dbMetrics
}

func newQueryTracer(m dbMetrics) *queryTracer {
	return &queryTracer{Tracer: otelpgx.NewTracer(otelpgx.WithMeterProvider(noop.NewMeterProvider())), metrics: m}
}

// Only fixed SQL identifiers become metric labels; arguments are never inspected.
func queryLabels(sql string) (string, string) {
	words := strings.Fields(strings.ToUpper(sql))
	if len(words) == 0 {
		return "UNKNOWN", ""
	}
	collection := ""
	for _, word := range words {
		switch word {
		case "TRIPS":
			collection = "trips"
		case "TRIP_POSITIONS":
			collection = "trip_positions"
		case "TRIP_STATUS_HISTORY":
			collection = "trip_status_history"
		}
	}
	return words[0], collection
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	op, table := queryLabels(data.SQL)
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("db.operation.name", op), attribute.String("db.collection.name", table))
	return context.WithValue(ctx, queryKey{}, queryInfo{started: time.Now(), operation: op, collection: table})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	info, _ := ctx.Value(queryKey{}).(queryInfo)
	errType := ""
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		errType = "db_error"
		var pgErr *pgconn.PgError
		if errors.As(data.Err, &pgErr) {
			errType = pgErr.Code
		}
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", errType))
	}
	if t.metrics != nil {
		t.metrics.RecordDBDuration(ctx, time.Since(info.started).Seconds(), info.operation, info.collection, errType)
	}
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}
