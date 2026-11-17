package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
)

func (r *Repository) InsertTrip(ctx context.Context, t *domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.Insert("trips").Columns(
		"id", "user_id", "driver_id", "price", "status",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude", "started_at",
	).Values(t.ID, t.UserID, t.DriverID, t.Price, string(t.Status), t.StartPoint.Latitude, t.StartPoint.Longitude, t.EndPoint.Latitude, t.EndPoint.Longitude, t.StartedAt).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}
	_, err = transaction.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_driver_active_unique_idx" {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus domain.Status, changedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.Insert("trip_status_history").Columns("trip_id", "from_status", "to_status", "reason", "changed_at").Values(tripID, fromStatus, string(toStatus), nil, changedAt).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history: %w", err)
	}
	if _, err := transaction.GetExecutor(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, t *domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.Update("trips").Set("status", string(t.Status)).Set("finished_at", t.FinishedAt).Set("updated_at", t.FinishedAt).Where(sq.Eq{"id": t.ID}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build update trip status: %w", err)
	}
	if _, err := transaction.GetExecutor(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("update trip status: %w", err)
	}
	return nil
}
