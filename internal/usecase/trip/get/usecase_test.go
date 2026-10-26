package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/get"
)

func TestUsecase_Execute(t *testing.T) {
	t.Parallel()
	id := uuid.New()

	t.Run("not found returns ErrTripNotFound", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)

		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, trip.ErrTripNotFound)

		uc := get.New(repo)
		_, err := uc.Execute(context.Background(), id)
		if !errors.Is(err, trip.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got %v", err)
		}
	})
}
