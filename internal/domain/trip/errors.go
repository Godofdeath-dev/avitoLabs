package trip

import (
	"errors"
	"fmt"
)

const invalidCoordinatesReason = "coordinates are out of range"

var (
	ErrPositionTripCompleted = errors.New("cannot save a position for a completed trip")
	ErrTripNotFound          = errors.New("trip not found")
	ErrTripCompleted         = errors.New("trip already completed")
	ErrDriverBusy            = errors.New("driver already has an active trip")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid field %s: %s", e.Field, e.Reason)
}
