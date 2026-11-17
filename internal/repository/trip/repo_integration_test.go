//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		t.Fatalf("database unavailable: %v", err)
	}
	return pool
}

func TestRepository_TransactionRollback(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	txManager := transaction.NewManager(pool, 3*time.Second)
	repo := triprepo.New(pool, 3*time.Second)

	coord := trip.Coordinates{Latitude: 55.75, Longitude: 37.61}

	// Создаем тестовую поездку
	newTrip, err := trip.NewTrip(uuid.New(), uuid.New(), 1000, coord, coord, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create domain trip: %v", err)
	}

	// Функция очистки: удаляем только ту поездку, которую создал этот тест
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM trip_status_history WHERE trip_id = $1", newTrip.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM trips WHERE id = $1", newTrip.ID)
	})

	// Сценарий 8: INSERT в первой таблице успешен, на второй операции ошибка -> полный откат
	err = txManager.Do(ctx, func(txCtx context.Context) error {
		// Первая успешная вставка
		if err := repo.InsertTrip(txCtx, newTrip); err != nil {
			return err
		}

		// First write a valid history entry, then violate its status constraint.
		if err := repo.AddStatusHistory(txCtx, newTrip.ID, nil, trip.StatusActive, time.Now()); err != nil {
			return err
		}
		return repo.AddStatusHistory(txCtx, newTrip.ID, nil, trip.Status("invalid"), time.Now())
	})

	// Ожидаем, что транзакция вернет ошибку
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("expected history CHECK violation, got %v", err)
	}

	// Проверяем, что запись в trips откатилась и в базе её нет
	var historyCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id=$1", newTrip.ID).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 0 {
		t.Fatalf("history not rolled back: %d", historyCount)
	}
	_, err = repo.GetByID(ctx, newTrip.ID)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound after rollback, got %v", err)
	}
}

func TestRepository_Lifecycle(t *testing.T) {
	pool := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := triprepo.New(pool, 3*time.Second)
	tx := transaction.NewManager(pool, 3*time.Second)
	coord := trip.Coordinates{Latitude: 55, Longitude: 37}
	item, err := trip.NewTrip(uuid.New(), uuid.New(), 0, coord, coord, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE driver_id=$1", item.DriverID); err != nil {
			t.Error(err)
		}
	})
	if err := tx.Do(ctx, func(ctx context.Context) error {
		if err := repo.InsertTrip(ctx, item); err != nil {
			return err
		}
		return repo.AddStatusHistory(ctx, item.ID, nil, trip.StatusActive, item.StartedAt)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, item.ID)
	if err != nil || got.ID != item.ID || got.Price != 0 || got.Status != trip.StatusActive {
		t.Fatalf("read trip: %+v %v", got, err)
	}
	other, err := trip.NewTrip(uuid.New(), item.DriverID, 100, coord, coord, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertTrip(ctx, other); !errors.Is(err, trip.ErrDriverBusy) {
		t.Fatalf("expected busy driver, got %v", err)
	}
	if err := repo.InsertTrip(ctx, item); errors.Is(err, trip.ErrDriverBusy) || err == nil {
		t.Fatalf("primary-key conflict must not become driver_busy: %v", err)
	}
	if err := tx.Do(ctx, func(ctx context.Context) error {
		locked, err := repo.GetByIDForUpdate(ctx, item.ID)
		if err != nil {
			return err
		}
		if err := locked.Finish(time.Now()); err != nil {
			return err
		}
		if err := repo.UpdateStatus(ctx, locked); err != nil {
			return err
		}
		from := string(trip.StatusActive)
		return repo.AddStatusHistory(ctx, item.ID, &from, trip.StatusCompleted, *locked.FinishedAt)
	}); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, item.ID)
	if err != nil || got.Status != trip.StatusCompleted || got.FinishedAt == nil {
		t.Fatalf("finished trip: %+v %v", got, err)
	}
	if err := repo.InsertTrip(ctx, other); err != nil {
		t.Fatalf("driver must be available after finish: %v", err)
	}
}
