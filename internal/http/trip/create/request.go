package create

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

type requestCoordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type requestBody struct {
	UserID     *uuid.UUID          `json:"user_id"`
	DriverID   *uuid.UUID          `json:"driver_id"`
	Price      *int64              `json:"price"`
	StartPoint *requestCoordinates `json:"start_point"`
	EndPoint   *requestCoordinates `json:"end_point"`
}

func decodeRequest(reader io.Reader) (api.CreateTripJSONRequestBody, error) {
	var body requestBody
	var result api.CreateTripJSONRequestBody
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return result, fmt.Errorf("decode request: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return result, fmt.Errorf("request must contain exactly one JSON object")
	}
	if body.UserID == nil || body.DriverID == nil || body.Price == nil || body.StartPoint == nil || body.EndPoint == nil {
		return result, fmt.Errorf("missing required field")
	}
	if body.StartPoint.Latitude == nil || body.StartPoint.Longitude == nil || body.EndPoint.Latitude == nil || body.EndPoint.Longitude == nil {
		return result, fmt.Errorf("missing coordinate")
	}
	result.UserId = *body.UserID
	result.DriverId = *body.DriverID
	result.Price = *body.Price
	result.StartPoint = api.Coordinates{Latitude: *body.StartPoint.Latitude, Longitude: *body.StartPoint.Longitude}
	result.EndPoint = api.Coordinates{Latitude: *body.EndPoint.Latitude, Longitude: *body.EndPoint.Longitude}
	return result, nil
}
