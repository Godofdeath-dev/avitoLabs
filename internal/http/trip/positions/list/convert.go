package list

import (
	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

func toResponse(p trip.Position) api.TripPosition {
	return api.TripPosition{Id: p.ID, TripId: p.TripID, Coordinates: api.Coordinates{Latitude: p.Coordinates.Latitude, Longitude: p.Coordinates.Longitude}, RecordedAt: p.RecordedAt}
}
