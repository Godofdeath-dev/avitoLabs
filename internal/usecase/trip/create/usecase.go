package create

import (
	"context"
	"log/slog"
	"time"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
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

func (u *Usecase) Execute(ctx context.Context, in model.CreateTripInput) (*trip.Trip, error) {
	now := time.Now().UTC()
	newTrip, err := trip.NewTrip(in.UserID, in.DriverID, in.Price, in.StartPoint, in.EndPoint, now)
	if err != nil {
		return nil, err
	}

	err = u.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := u.repo.InsertTrip(txCtx, newTrip); err != nil {
			return err
		}
		return u.repo.AddStatusHistory(txCtx, newTrip.ID, nil, newTrip.Status, newTrip.StartedAt)
	})
	if err != nil {
		return nil, err
	}

	if u.metrics != nil {
		u.metrics.IncTripsCreated(ctx)
	}
	if u.logger != nil {
		u.logger.InfoContext(ctx, "trip created", slog.String("trip_id", newTrip.ID.String()))
	}
	return newTrip, nil
}
