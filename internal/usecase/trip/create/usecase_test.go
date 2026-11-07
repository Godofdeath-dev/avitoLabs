package create_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
	"github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/create"
)

func TestUsecase_Execute(t *testing.T) {
	t.Parallel()
	validInput := model.CreateTripInput{
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		Price:      1200,
		StartPoint: trip.Coordinates{Latitude: 55.7, Longitude: 37.6},
		EndPoint:   trip.Coordinates{Latitude: 55.8, Longitude: 37.7},
	}

	t.Run("success create trip", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		m := NewMockmetrics(ctrl)

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().InsertTrip(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any(), nil, trip.StatusActive, gomock.Any()).Return(nil)
		m.EXPECT().IncTripsCreated(gomock.Any())

		uc := create.New(repo, tx, m, nil)
		res, err := uc.Execute(context.Background(), validInput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != trip.StatusActive {
			t.Errorf("got status %s, want active", res.Status)
		}
	})

	t.Run("driver busy returns ErrDriverBusy", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		m := NewMockmetrics(ctrl)

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().InsertTrip(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)

		uc := create.New(repo, tx, m, nil)
		_, err := uc.Execute(context.Background(), validInput)
		if !errors.Is(err, trip.ErrDriverBusy) {
			t.Fatalf("expected ErrDriverBusy, got %v", err)
		}
	})
}
