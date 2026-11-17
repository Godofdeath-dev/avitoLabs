package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type SavePositionInput struct {
	TripID      uuid.UUID
	Coordinates trip.Coordinates
	RecordedAt  time.Time
}
type StaleTrip struct {
	ID       uuid.UUID
	DriverID uuid.UUID
}
