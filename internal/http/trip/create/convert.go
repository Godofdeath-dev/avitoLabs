package create

import (
	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

func toInput(body api.CreateTripJSONRequestBody) model.CreateTripInput {
	return model.CreateTripInput{
		UserID:   body.UserId,
		DriverID: body.DriverId,
		Price:    body.Price,
		StartPoint: trip.Coordinates{
			Latitude:  body.StartPoint.Latitude,
			Longitude: body.StartPoint.Longitude,
		},
		EndPoint: trip.Coordinates{
			Latitude:  body.EndPoint.Latitude,
			Longitude: body.EndPoint.Longitude,
		},
	}
}

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
