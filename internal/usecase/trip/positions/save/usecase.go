package save

import (
	"context"
	"fmt"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

type Usecase struct {
	repo    tripRepository
	tx      txManager
	pending canceller
	metrics metrics
	clock   clock
}

func New(repo tripRepository, tx txManager, pending canceller, m metrics, c clock) *Usecase {
	return &Usecase{repo: repo, tx: tx, pending: pending, metrics: m, clock: c}
}

func (u *Usecase) Execute(ctx context.Context, in model.SavePositionInput) (*trip.Position, error) {
	p, err := trip.NewPosition(in.TripID, in.Coordinates, in.RecordedAt, u.clock.Now())
	if err != nil {
		return nil, err
	}
	// Точка и время последней координаты сохраняются вместе либо вместе откатываются.
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		// Блокировка строки не даёт одновременно завершить поездку.
		t, err := u.repo.GetByIDForUpdate(ctx, in.TripID)
		if err != nil {
			return err
		}
		if t.Status != trip.StatusActive {
			return trip.ErrPositionTripCompleted
		}
		id, err := u.repo.InsertPosition(ctx, p)
		if err != nil {
			return err
		}
		p.ID = id
		return u.repo.AdvanceLastPosition(ctx, in.TripID, p.RecordedAt)
	})
	if err != nil {
		return nil, fmt.Errorf("save position transaction: %w", err)
	}
	// Отменяем опрос только после успешного commit.
	u.pending.Cancel(in.TripID)
	u.metrics.PositionSaved(ctx)
	return p, nil
}
