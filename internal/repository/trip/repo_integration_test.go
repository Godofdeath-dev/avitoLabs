//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
	triprepo "github.com/Godofdeath-dev/avitoLabs/internal/repository/trip"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	_, err = pool.Exec(ctx, "TRUNCATE TABLE trip_status_history, trips CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE trip_status_history, trips CASCADE")
		pool.Close()
	})
	return pool
}

func TestRepository_TransactionRollback(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	txManager := transaction.NewManager(pool)
	repo := triprepo.New(pool, nil)

	coord := trip.Coordinates{Latitude: 55.75, Longitude: 37.61}
	newTrip, err := trip.NewTrip(uuid.New(), uuid.New(), 1000, coord, coord, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create domain trip: %v", err)
	}

	// Сценарий 8: INSERT в первой таблице успешен, на второй операции ошибка -> полный откат
	simulatedErr := errors.New("second insert failed")
	err = txManager.Do(ctx, func(txCtx context.Context) error {
		if err := repo.InsertTrip(txCtx, newTrip); err != nil {
			return err
		}
		return simulatedErr
	})
	if !errors.Is(err, simulatedErr) {
		t.Fatalf("expected simulatedErr, got %v", err)
	}

	// Проверяем, что запись в trips откатилась и в базе её нет
	_, err = repo.GetByID(ctx, newTrip.ID)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound after rollback, got %v", err)
	}
}
