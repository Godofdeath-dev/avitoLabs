package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/finish"
)

func TestUsecase_Execute(t *testing.T) {
	t.Parallel()
	id := uuid.New()

	t.Run("finish active trip success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		m := NewMockmetrics(ctrl)

		activeTrip := &trip.Trip{ID: id, Status: trip.StatusActive, StartedAt: time.Now().UTC()}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().GetByIDForUpdate(gomock.Any(), id).Return(activeTrip, nil)
		repo.EXPECT().UpdateStatus(gomock.Any(), activeTrip).Return(nil)
		repo.EXPECT().AddStatusHistory(gomock.Any(), id, gomock.Any(), trip.StatusCompleted, gomock.Any()).Return(nil)
		m.EXPECT().IncTripsCompleted(gomock.Any())

		uc := finish.New(repo, tx, m, nil)
		res, err := uc.Execute(context.Background(), id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != trip.StatusCompleted {
			t.Errorf("got status %s, want completed", res.Status)
		}
	})

	t.Run("finish already completed trip returns ErrTripCompleted", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		m := NewMockmetrics(ctrl)

		completedTrip := &trip.Trip{ID: id, Status: trip.StatusCompleted}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().GetByIDForUpdate(gomock.Any(), id).Return(completedTrip, nil)

		uc := finish.New(repo, tx, m, nil)
		_, err := uc.Execute(context.Background(), id)
		if !errors.Is(err, trip.ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got %v", err)
		}
	})
}
