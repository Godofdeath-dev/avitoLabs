//go:build integration

package trip_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
	repo "github.com/Godofdeath-dev/avitoLabs/internal/repository/trip"
)

func storedTrip(t *testing.T, pool *pgxpool.Pool, at time.Time) *trip.Trip {
	t.Helper()
	item, err := trip.NewTrip(uuid.New(), uuid.New(), 0, trip.Coordinates{}, trip.Coordinates{}, at)
	require.NoError(t, err)
	r := repo.New(pool, time.Second)
	require.NoError(t, r.InsertTrip(context.Background(), item))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, "DELETE FROM trips WHERE id=$1", item.ID)
		require.NoError(t, err)
	})
	return item
}

func TestPositionsAtomicityAndOrdering(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	r := repo.New(pool, time.Second)
	tx := transaction.NewManager(pool, time.Second)
	now := time.Now().UTC().Truncate(time.Microsecond)
	item := storedTrip(t, pool, now.Add(-time.Hour))
	var ids []int64
	for _, at := range []time.Time{now, now.Add(-time.Minute), now} {
		p, err := trip.NewPosition(item.ID, trip.Coordinates{}, at, now)
		require.NoError(t, err)
		require.NoError(t, tx.Do(ctx, func(ctx context.Context) error {
			id, err := r.InsertPosition(ctx, p)
			if err != nil {
				return err
			}
			ids = append(ids, id)
			return r.AdvanceLastPosition(ctx, item.ID, at)
		}))
	}
	got, err := r.GetByID(ctx, item.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastPositionAt)
	require.True(t, now.Equal(*got.LastPositionAt), "late position moved freshness backwards")
	points, err := r.ListPositions(ctx, item.ID)
	require.NoError(t, err)
	require.Len(t, points, 3)
	require.Equal(t, []int64{ids[1], ids[0], ids[2]}, []int64{points[0].ID, points[1].ID, points[2].ID})
	p, err := trip.NewPosition(item.ID, trip.Coordinates{}, now.Add(time.Second), now)
	require.NoError(t, err)
	err = tx.Do(ctx, func(ctx context.Context) error {
		if _, err := r.InsertPosition(ctx, p); err != nil {
			return err
		}
		return r.AdvanceLastPosition(ctx, uuid.New(), p.RecordedAt)
	})
	require.ErrorIs(t, err, trip.ErrTripNotFound)
	points, err = r.ListPositions(ctx, item.ID)
	require.NoError(t, err)
	require.Len(t, points, 3, "position insert was not rolled back")
	got, err = r.GetByID(ctx, item.ID)
	require.NoError(t, err)
	require.True(t, now.Equal(*got.LastPositionAt))
}

func TestStaleSelection(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	r := repo.New(pool, time.Second)
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := storedTrip(t, pool, now.Add(-3*time.Minute))
	stale := storedTrip(t, pool, now.Add(-4*time.Minute))
	fresh := storedTrip(t, pool, now.Add(-4*time.Minute))
	completed := storedTrip(t, pool, now.Add(-4*time.Minute))
	boundary := storedTrip(t, pool, now.Add(-time.Minute))
	require.NoError(t, r.AdvanceLastPosition(ctx, stale.ID, now.Add(-2*time.Minute)))
	require.NoError(t, r.AdvanceLastPosition(ctx, fresh.ID, now))
	require.NoError(t, completed.Finish(now))
	require.NoError(t, r.UpdateStatus(ctx, completed))
	items, err := r.FindStale(ctx, now, time.Minute)
	require.NoError(t, err)
	owned := map[uuid.UUID]bool{old.ID: true, stale.ID: true, fresh.ID: true, completed.ID: true, boundary.ID: true}
	var selected []uuid.UUID
	for _, item := range items {
		if owned[item.ID] {
			selected = append(selected, item.ID)
		}
	}
	require.Equal(t, []uuid.UUID{old.ID, stale.ID}, selected)
}

func TestStaleIndexPlan(t *testing.T) {
	pool := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	fixtureUser := uuid.New()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, "DELETE FROM trips WHERE user_id=$1", fixtureUser)
		require.NoError(t, err)
	})
	_, err := pool.Exec(ctx, `INSERT INTO trips(id,user_id,driver_id,price,status,start_latitude,start_longitude,end_latitude,end_longitude,started_at)
SELECT gen_random_uuid(),$1,gen_random_uuid(),0,'active',0,0,0,0,now()-CASE WHEN n<=10 THEN interval '2 hours' ELSE interval '10 seconds' END FROM generate_series(1,6000) AS n`, fixtureUser)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "ANALYZE trips")
	require.NoError(t, err)
	rows, err := pool.Query(ctx, `EXPLAIN (ANALYZE, BUFFERS) SELECT id,driver_id FROM trips WHERE status='active' AND COALESCE(last_position_at,started_at)<$1::timestamptz-($2*interval '1 second') ORDER BY COALESCE(last_position_at,started_at),id`, time.Now().UTC(), 60.0)
	require.NoError(t, err)
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	plan := strings.Join(lines, "\n")
	t.Log(plan)
	require.Contains(t, plan, "trips_status_position_time_idx")
	require.Contains(t, plan, "Index Scan")
}
