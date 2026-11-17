package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type Usecase struct{ repo tripRepository }

func New(repo tripRepository) *Usecase { return &Usecase{repo: repo} }
func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) ([]trip.Position, error) {
	// Отличаем отсутствующую поездку (404) от существующей без точек (пустой список).
	if _, err := u.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	return u.repo.ListPositions(ctx, id)
}
