package trip

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

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
	if r.metrics == nil {
		return
	}
	errType := ""
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			errType = pgErr.Code
		} else {
			errType = "db_error"
		}
	}
	r.metrics.RecordDBDuration(ctx, time.Since(start).Seconds(), op, table, errType)
}
