//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type (
	tripRepository interface {
		InsertTrip(ctx context.Context, t *trip.Trip) error
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus trip.Status, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metrics interface {
		IncTripsCreated(ctx context.Context)
	}
)
