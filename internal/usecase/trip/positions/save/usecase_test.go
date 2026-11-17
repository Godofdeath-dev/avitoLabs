package save_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
	"github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/positions/save"
)

const successfulSave = "success"

func TestSavePosition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name               string
		status             trip.Status
		repoErr, commitErr error
	}{
		{name: successfulSave, status: trip.StatusActive}, {name: "completed", status: trip.StatusCompleted}, {name: "missing", repoErr: trip.ErrTripNotFound}, {name: "commit failure", status: trip.StatusActive, commitErr: errors.New("commit failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			tx := NewMocktxManager(ctrl)
			pending := NewMockcanceller(ctrl)
			m := NewMockmetrics(ctrl)
			clock := NewMockclock(ctrl)
			id := uuid.New()
			clock.EXPECT().Now().Return(now)
			var committed bool
			repo.EXPECT().GetByIDForUpdate(gomock.Any(), id).Return(&trip.Trip{ID: id, Status: tc.status}, tc.repoErr)
			tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				if err := fn(ctx); err != nil {
					return err
				}
				if tc.commitErr != nil {
					return tc.commitErr
				}
				committed = true
				return nil
			})
			if tc.repoErr == nil && tc.status == trip.StatusActive {
				repo.EXPECT().InsertPosition(gomock.Any(), gomock.Any()).Return(int64(42), nil)
				repo.EXPECT().AdvanceLastPosition(gomock.Any(), id, now).Return(nil)
			}
			if tc.name == successfulSave {
				pending.EXPECT().Cancel(id).Do(func(uuid.UUID) { require.True(t, committed, "cancel before commit") })
				m.EXPECT().PositionSaved(gomock.Any()).Do(func(context.Context) { require.True(t, committed, "metric before commit") })
			}
			result, err := save.New(repo, tx, pending, m, clock).Execute(context.Background(), model.SavePositionInput{TripID: id, RecordedAt: now})
			switch tc.name {
			case successfulSave:
				require.NoError(t, err)
				require.Equal(t, int64(42), result.ID)
			case "completed":
				require.ErrorIs(t, err, trip.ErrPositionTripCompleted)
			case "missing":
				require.ErrorIs(t, err, trip.ErrTripNotFound)
			default:
				require.ErrorIs(t, err, tc.commitErr)
			}
		})
	}
}

func TestInvalidPositionDoesNotStartTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	pending := NewMockcanceller(ctrl)
	m := NewMockmetrics(ctrl)
	clock := NewMockclock(ctrl)
	clock.EXPECT().Now().Return(time.Now())
	_, err := save.New(repo, tx, pending, m, clock).Execute(context.Background(), model.SavePositionInput{})
	require.Error(t, err)
}
