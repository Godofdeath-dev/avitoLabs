package trip

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Trip struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	Price      int64
	Status     Status
	StartPoint Coordinates
	EndPoint   Coordinates
	StartedAt  time.Time
	FinishedAt *time.Time
}

func NewTrip(userID, driverID uuid.UUID, price int64, start, end Coordinates, now time.Time) (*Trip, error) {
	if price <= 0 {
		return nil, &ValidationError{Field: "price", Reason: "price must be greater than zero"}
	}
	if !validCoords(start) {
		return nil, &ValidationError{Field: "start_point", Reason: "coordinates are out of range"}
	}
	if !validCoords(end) {
		return nil, &ValidationError{Field: "end_point", Reason: "coordinates are out of range"}
	}

	return &Trip{
		ID:         uuid.New(),
		UserID:     userID,
		DriverID:   driverID,
		Price:      price,
		Status:     StatusActive,
		StartPoint: start,
		EndPoint:   end,
		StartedAt:  now.UTC(),
	}, nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	utc := at.UTC()
	t.FinishedAt = &utc
	return nil
}

func validCoords(c Coordinates) bool {
	return c.Latitude >= -90 && c.Latitude <= 90 && c.Longitude >= -180 && c.Longitude <= 180
}
