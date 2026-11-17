package list_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/positions/list"
)

func TestListPositions(t *testing.T) {
	t.Parallel()
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty route", true: "missing trip"}[missing], func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			id := uuid.New()
			if missing {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, trip.ErrTripNotFound)
			} else {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&trip.Trip{ID: id}, nil)
				repo.EXPECT().ListPositions(gomock.Any(), id).Return([]trip.Position{}, nil)
			}
			result, err := list.New(repo).Execute(context.Background(), id)
			if missing {
				require.ErrorIs(t, err, trip.ErrTripNotFound)
			} else {
				require.NoError(t, err)
				require.Empty(t, result)
			}
		})
	}
}
