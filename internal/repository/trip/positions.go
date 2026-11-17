package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
)

func (r *Repository) InsertPosition(ctx context.Context, p *domain.Position) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.Insert("trip_positions").Columns("trip_id", "latitude", "longitude", "recorded_at").Values(p.TripID, p.Coordinates.Latitude, p.Coordinates.Longitude, p.RecordedAt).Suffix("RETURNING id").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build insert position: %w", err)
	}
	var id int64
	if err := transaction.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert position: %w", err)
	}
	return id, nil
}

func (r *Repository) AdvanceLastPosition(ctx context.Context, id uuid.UUID, at time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	// GREATEST не позволяет опоздавшей точке уменьшить last_position_at.
	query, args, err := sq.Update("trips").Set("last_position_at", sq.Expr("GREATEST(last_position_at, ?)", at)).Set("updated_at", sq.Expr("now()")).Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build update last position: %w", err)
	}
	tag, err := transaction.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrTripNotFound
	}
	return nil
}

func (r *Repository) ListPositions(ctx context.Context, id uuid.UUID) ([]domain.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	// id задаёт однозначный порядок для точек с одинаковым recorded_at.
	query, args, err := sq.Select("id", "trip_id", "latitude", "longitude", "recorded_at").From("trip_positions").Where(sq.Eq{"trip_id": id}).OrderBy("recorded_at ASC", "id ASC").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions: %w", err)
	}
	rows, err := transaction.GetExecutor(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()
	positions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Position, error) {
		var p domain.Position
		err := row.Scan(&p.ID, &p.TripID, &p.Coordinates.Latitude, &p.Coordinates.Longitude, &p.RecordedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan positions: %w", err)
	}
	return positions, nil
}
