//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type tripRepository interface {
	GetByID(context.Context, uuid.UUID) (*trip.Trip, error)
	ListPositions(context.Context, uuid.UUID) ([]trip.Position, error)
}
