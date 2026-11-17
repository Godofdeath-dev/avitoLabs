//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type usecase interface {
	Execute(context.Context, uuid.UUID) ([]trip.Position, error)
}
