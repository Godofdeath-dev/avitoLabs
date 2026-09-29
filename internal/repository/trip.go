package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

var (
	ErrNotFound        = errors.New("trip not found")
	ErrAlreadyFinished = errors.New("trip already completed")
)

type TripRepository struct {
	pool *pgxpool.Pool
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip api.Trip) (*api.Trip, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	queryTrip := `
		INSERT INTO trips (
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.Exec(ctx, queryTrip,
		trip.Id,
		trip.UserId,
		trip.DriverId,
		trip.Price,
		string(trip.Status),
		trip.StartPoint.Latitude,
		trip.StartPoint.Longitude,
		trip.EndPoint.Latitude,
		trip.EndPoint.Longitude,
		trip.StartedAt,
	)
	if err != nil {
		return nil, err
	}

	queryHistory := `
		INSERT INTO trip_status_history (trip_id, from_status, to_status, changed_at)
		VALUES ($1, NULL, $2, $3)
	`
	_, err = tx.Exec(ctx, queryHistory, trip.Id, string(trip.Status), trip.StartedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &trip, nil
}

func (r *TripRepository) GetTripByID(ctx context.Context, id api.TripId) (*api.Trip, error) {
	query := `
		SELECT 
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at, finished_at
		FROM trips
		WHERE id = $1
	`

	var trip api.Trip
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&trip.Id,
		&trip.UserId,
		&trip.DriverId,
		&trip.Price,
		&trip.Status,
		&trip.StartPoint.Latitude,
		&trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude,
		&trip.EndPoint.Longitude,
		&trip.StartedAt,
		&trip.FinishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &trip, nil
}

func (r *TripRepository) FinishTrip(ctx context.Context, id api.TripId, finishedAt time.Time) (*api.Trip, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	queryCheck := `SELECT status FROM trips WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(ctx, queryCheck, id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if currentStatus == string(api.Completed) {
		return nil, ErrAlreadyFinished
	}

	queryUpdate := `
		UPDATE trips
		SET status = $2, finished_at = $3, updated_at = $3
		WHERE id = $1
		RETURNING 
			id, user_id, driver_id, price, status,
			start_latitude, start_longitude, end_latitude, end_longitude,
			started_at, finished_at
	`

	var trip api.Trip
	err = tx.QueryRow(ctx, queryUpdate, id, string(api.Completed), finishedAt).Scan(
		&trip.Id,
		&trip.UserId,
		&trip.DriverId,
		&trip.Price,
		&trip.Status,
		&trip.StartPoint.Latitude,
		&trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude,
		&trip.EndPoint.Longitude,
		&trip.StartedAt,
		&trip.FinishedAt,
	)
	if err != nil {
		return nil, err
	}

	queryHistory := `
		INSERT INTO trip_status_history (trip_id, from_status, to_status, changed_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, queryHistory, id, currentStatus, string(api.Completed), finishedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &trip, nil
}
