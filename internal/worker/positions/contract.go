//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package positions

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

type repository interface {
	FindStale(context.Context, time.Time, time.Duration) ([]model.StaleTrip, error)
}
type pushGateway interface {
	RequestPosition(context.Context, model.StaleTrip) error
}
type metrics interface {
	StaleCheck(context.Context, string)
	PositionRequest(context.Context, string, float64)
}

// В тестах часы и тикер можно заменить управляемой реализацией.
type clock interface {
	Now() time.Time
	NewTicker(time.Duration) clockwork.Ticker
}
