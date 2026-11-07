package get

import (
	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

func toResponse(t *trip.Trip) api.Trip {
	return api.Trip{
		Id:       t.ID,
		UserId:   t.UserID,
		DriverId: t.DriverID,
		Price:    t.Price,
		Status:   api.TripStatus(t.Status),
		StartPoint: api.Coordinates{
			Latitude:  t.StartPoint.Latitude,
			Longitude: t.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndPoint.Latitude,
			Longitude: t.EndPoint.Longitude,
		},
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}
