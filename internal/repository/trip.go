package repository

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

// Инициализируем билдер запросов squirrel
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var (
	ErrNotFound        = errors.New("trip not found") // todo попозже заменю на сгенерированные в api.gen
	ErrAlreadyFinished = errors.New("trip already completed")
	ErrDriverBusy      = errors.New("driver_busy") // Добавлена доменная ошибка для конфликта
)

type TripRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// Добавлен timeout в конструктор
func NewTripRepository(pool *pgxpool.Pool, timeout time.Duration) *TripRepository {
	return &TripRepository{
		pool:    pool,
		timeout: timeout,
	}
}

// InsertTrip выполняет вставку только в таблицу trips
func (r *TripRepository) InsertTrip(ctx context.Context, trip api.Trip) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	executor := getExecutor(queryCtx, r.pool)

	sql, args, err := psql.Insert("trips").
		Columns("id", "user_id", "driver_id", "price", "status",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"started_at").
		Values(trip.Id, trip.UserId, trip.DriverId, trip.Price, string(trip.Status),
			trip.StartPoint.Latitude, trip.StartPoint.Longitude,
			trip.EndPoint.Latitude, trip.EndPoint.Longitude,
			trip.StartedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = executor.Exec(queryCtx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 - код ошибки PostgreSQL для нарушения уникального ограничения (unique_violation)
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDriverBusy
		}
		return err
	}
	return nil
}

// AddStatusHistory выполняет запись в таблицу trip_status_history
func (r *TripRepository) AddStatusHistory(ctx context.Context, tripID api.TripId, fromStatus *string, toStatus api.TripStatus, changedAt time.Time) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	executor := getExecutor(queryCtx, r.pool)

	sql, args, err := psql.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "changed_at").
		Values(tripID, fromStatus, string(toStatus), changedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = executor.Exec(queryCtx, sql, args...)
	return err
}

// GetTripByID читает поездку (в транзакции или через пул в зависимости от контекста)
func (r *TripRepository) GetTripByID(ctx context.Context, id api.TripId) (*api.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	executor := getExecutor(queryCtx, r.pool)

	sql, args, err := psql.Select(
		"id", "user_id", "driver_id", "price", "status",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"started_at", "finished_at").
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var trip api.Trip
	err = executor.QueryRow(queryCtx, sql, args...).Scan(
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

// UpdateTripCompleted переводит поездку в статус completed с блокировкой строки
func (r *TripRepository) UpdateTripCompleted(ctx context.Context, id api.TripId, finishedAt time.Time) (*api.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	executor := getExecutor(queryCtx, r.pool)

	// 1. Проверяем текущий статус с блокировкой строки
	sqlCheck, argsCheck, err := psql.Select("status").
		From("trips").
		Where(sq.Eq{"id": id}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return nil, err
	}

	var currentStatus string
	err = executor.QueryRow(queryCtx, sqlCheck, argsCheck...).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if currentStatus == string(api.Completed) {
		return nil, ErrAlreadyFinished
	}

	// 2. Обновляем статус и возвращаем обновленную строку
	sqlUpdate, argsUpdate, err := psql.Update("trips").
		Set("status", string(api.Completed)).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING id, user_id, driver_id, price, status, start_latitude, start_longitude, end_latitude, end_longitude, started_at, finished_at").
		ToSql()
	if err != nil {
		return nil, err
	}

	var trip api.Trip
	err = executor.QueryRow(queryCtx, sqlUpdate, argsUpdate...).Scan(
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

	return &trip, nil
}
