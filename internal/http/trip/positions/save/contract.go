//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package save

import (
	"context"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

type usecase interface {
	Execute(context.Context, model.SavePositionInput) (*trip.Position, error)
}
