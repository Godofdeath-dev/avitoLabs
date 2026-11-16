package model

import (
	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

type CreateTripInput struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	Price      int64
	StartPoint trip.Coordinates
	EndPoint   trip.Coordinates
}
