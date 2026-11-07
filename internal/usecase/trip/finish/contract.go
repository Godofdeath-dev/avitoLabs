//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type (
	tripRepository interface {
		GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*trip.Trip, error)
		UpdateStatus(ctx context.Context, t *trip.Trip) error
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus trip.Status, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metrics interface {
		IncTripsCompleted(ctx context.Context)
	}
)
