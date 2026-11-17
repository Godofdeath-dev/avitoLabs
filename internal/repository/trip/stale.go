package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

func (r *Repository) FindStale(ctx context.Context, now time.Time, staleAfter time.Duration) ([]model.StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	// Без координат считаем возраст от начала поездки; отбор и сортировка идут в БД.
	query, args, err := sq.Select("id", "driver_id").From("trips").Where("status = 'active'").Where("COALESCE(last_position_at, started_at) < ?::timestamptz - (? * interval '1 second')", now.UTC(), staleAfter.Seconds()).OrderBy("COALESCE(last_position_at, started_at) ASC", "id ASC").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build find stale trips: %w", err)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find stale trips: %w", err)
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.StaleTrip, error) {
		var t model.StaleTrip
		err := row.Scan(&t.ID, &t.DriverID)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan stale trips: %w", err)
	}
	return items, nil
}
