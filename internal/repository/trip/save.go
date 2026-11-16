package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
)

func (r *Repository) InsertTrip(ctx context.Context, t *domain.Trip) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := transaction.GetExecutor(ctx, r.pool)
	query := `
		INSERT INTO trips (
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = executor.Exec(ctx, query,
		t.ID,
		t.UserID,
		t.DriverID,
		t.Price,
		string(t.Status),
		t.StartPoint.Latitude,
		t.StartPoint.Longitude,
		t.EndPoint.Latitude,
		t.EndPoint.Longitude,
		t.StartedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_driver_active_unique_idx" {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus domain.Status, changedAt time.Time) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := transaction.GetExecutor(ctx, r.pool)
	query := `
		INSERT INTO trip_status_history (trip_id, from_status, to_status, reason, changed_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = executor.Exec(ctx, query, tripID, fromStatus, string(toStatus), nil, changedAt)
	if err != nil {
		return fmt.Errorf("write trip state: %w", err)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, t *domain.Trip) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := transaction.GetExecutor(ctx, r.pool)
	query := `
		UPDATE trips
		SET status = $2, finished_at = $3, updated_at = $3
		WHERE id = $1
	`
	_, err = executor.Exec(ctx, query, t.ID, string(t.Status), t.FinishedAt)
	if err != nil {
		return fmt.Errorf("write trip state: %w", err)
	}
	return nil
}
