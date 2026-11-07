package finish

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type Usecase struct {
	repo      tripRepository
	txManager txManager
	metrics   metrics
	logger    *slog.Logger
}

func New(repo tripRepository, tx txManager, m metrics, l *slog.Logger) *Usecase {
	return &Usecase{
		repo:      repo,
		txManager: tx,
		metrics:   m,
		logger:    l,
	}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (*trip.Trip, error) {
	now := time.Now().UTC()
	var currentTrip *trip.Trip

	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		var err error
		currentTrip, err = u.repo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		fromStatus := string(currentTrip.Status)
		if err := currentTrip.Finish(now); err != nil {
			return err
		}

		if err := u.repo.UpdateStatus(txCtx, currentTrip); err != nil {
			return err
		}

		return u.repo.AddStatusHistory(txCtx, currentTrip.ID, &fromStatus, currentTrip.Status, now)
	})
	if err != nil {
		return nil, err
	}

	if u.metrics != nil {
		u.metrics.IncTripsCompleted(ctx)
	}
	if u.logger != nil {
		u.logger.InfoContext(ctx, "trip completed", slog.String("trip_id", currentTrip.ID.String()))
	}
	return currentTrip, nil
}
