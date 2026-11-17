//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type tripRepository interface {
	GetByIDForUpdate(context.Context, uuid.UUID) (*trip.Trip, error)
	InsertPosition(context.Context, *trip.Position) (int64, error)
	AdvanceLastPosition(context.Context, uuid.UUID, time.Time) error
}
type txManager interface {
	Do(context.Context, func(context.Context) error) error
}
type (
	canceller interface{ Cancel(uuid.UUID) }
	metrics   interface{ PositionSaved(context.Context) }
	clock     interface{ Now() time.Time }
)
