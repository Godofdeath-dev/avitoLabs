package trip

import (
	"time"

	"github.com/google/uuid"
)

// Position — координата, зафиксированная устройством водителя.
type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
}

// NewPosition принимает текущее время снаружи, чтобы проверять даты без ожидания в тестах.
func NewPosition(tripID uuid.UUID, coordinates Coordinates, recordedAt, now time.Time) (*Position, error) {
	if !validCoords(coordinates) {
		return nil, &ValidationError{Field: "coordinates", Reason: invalidCoordinatesReason}
	}
	if recordedAt.IsZero() {
		return nil, &ValidationError{Field: "recorded_at", Reason: "recorded_at is required"}
	}
	if recordedAt.After(now.Add(time.Minute)) {
		return nil, &ValidationError{Field: "recorded_at", Reason: "recorded_at is more than one minute in the future"}
	}
	return &Position{TripID: tripID, Coordinates: coordinates, RecordedAt: recordedAt.UTC()}, nil
}
