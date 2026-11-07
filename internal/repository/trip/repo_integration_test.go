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

	// Закрываем пул после завершения всех тестов,
	// удаление данных больше не делаем на уровне всей БД.
	t.Cleanup(func() {
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

	// Создаем тестовую поездку
	testID := uuid.New()
	newTrip, err := trip.NewTrip(testID, uuid.New(), 1000, coord, coord, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create domain trip: %v", err)
	}

	// Функция очистки: удаляем только ту поездку, которую создал этот тест
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM trip_status_history WHERE trip_id = $1", testID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM trips WHERE id = $1", testID)
	})

	// Сценарий 8: INSERT в первой таблице успешен, на второй операции ошибка -> полный откат
	err = txManager.Do(ctx, func(txCtx context.Context) error {
		// Первая успешная вставка
		if err := repo.InsertTrip(txCtx, newTrip); err != nil {
			return err
		}

		// Имитируем реальную ошибку БД, пытаясь вставить ту же самую запись второй раз
		// Это вызовет нарушение уникальности (unique_violation)
		return repo.InsertTrip(txCtx, newTrip)
	})

	// Ожидаем, что транзакция вернет ошибку
	if err == nil {
		t.Fatalf("expected db error from second insert, got nil")
	}

	// Проверяем, что запись в trips откатилась и в базе её нет
	_, err = repo.GetByID(ctx, newTrip.ID)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound after rollback, got %v", err)
	}
}
