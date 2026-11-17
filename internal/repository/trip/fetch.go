package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
)

func selectTrip(id uuid.UUID) sq.SelectBuilder {
	return sq.Select("id", "user_id", "driver_id", "price", "status", "start_latitude", "start_longitude", "end_latitude", "end_longitude", "started_at", "finished_at").From("trips").Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar)
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := selectTrip(id).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get trip: %w", err)
	}
	return scanTrip(transaction.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...))
}

func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := selectTrip(id).Suffix("FOR UPDATE").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get trip for update: %w", err)
	}
	return scanTrip(transaction.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...))
}

func scanTrip(row pgx.Row) (*domain.Trip, error) {
	var t domain.Trip
	var status string
	err := row.Scan(&t.ID, &t.UserID, &t.DriverID, &t.Price, &status, &t.StartPoint.Latitude, &t.StartPoint.Longitude, &t.EndPoint.Latitude, &t.EndPoint.Longitude, &t.StartedAt, &t.FinishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	t.Status = domain.Status(status)
	return &t, nil
}
