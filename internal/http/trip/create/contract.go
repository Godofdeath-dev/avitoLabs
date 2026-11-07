//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, in model.CreateTripInput) (*trip.Trip, error)
}
