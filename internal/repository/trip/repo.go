package trip

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
)

type Repository struct {
	pool    *pgxpool.Pool
	metrics *telemetry.Metrics
}

func New(pool *pgxpool.Pool, m *telemetry.Metrics) *Repository {
	return &Repository{
		pool:    pool,
		metrics: m,
	}
}

func (r *Repository) recordMetric(ctx context.Context, start time.Time, op, table string, err error) {
	errType := ""
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			errType = pgErr.Code
		} else {
			errType = "db_error"
		}
	}

	// 1. Дополняем SQL-спан от otelpgx недостающими атрибутами
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String("db.collection.name", table))
		if errType != "" {
			span.SetAttributes(attribute.String("error.type", errType))
		}
	}

	if r.metrics == nil {
		return
	}
	r.metrics.RecordDBDuration(ctx, time.Since(start).Seconds(), op, table, errType)
}
