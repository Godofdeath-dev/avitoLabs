package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
)

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (_ *domain.Trip, err error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := transaction.GetExecutor(ctx, r.pool)
	query := `
		SELECT 
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at, finished_at
		FROM trips
		WHERE id = $1
	`
	return scanTrip(executor.QueryRow(ctx, query, id))
}

func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (_ *domain.Trip, err error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	executor := transaction.GetExecutor(ctx, r.pool)
	query := `
		SELECT 
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at, finished_at
		FROM trips
		WHERE id = $1
		FOR UPDATE
	`
	return scanTrip(executor.QueryRow(ctx, query, id))
}

func scanTrip(row pgx.Row) (*domain.Trip, error) {
	var t domain.Trip
	var statusStr string
	err := row.Scan(
		&t.ID,
		&t.UserID,
		&t.DriverID,
		&t.Price,
		&statusStr,
		&t.StartPoint.Latitude,
		&t.StartPoint.Longitude,
		&t.EndPoint.Latitude,
		&t.EndPoint.Longitude,
		&t.StartedAt,
		&t.FinishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	t.Status = domain.Status(statusStr)
	return &t, nil
}
