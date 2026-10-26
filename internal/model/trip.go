package model

import (
	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/google/uuid"
)

type CreateTripInput struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	Price      int64
	StartPoint trip.Coordinates
	EndPoint   trip.Coordinates
}
