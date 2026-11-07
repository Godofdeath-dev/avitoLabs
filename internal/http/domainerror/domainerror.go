package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

func Problem(err error) (int, api.Problem) {
	var valErr *trip.ValidationError
	switch {
	case errors.As(err, &valErr):
		detail := valErr.Error()
		return http.StatusBadRequest, api.Problem{
			Status: http.StatusBadRequest,
			Code:   "invalid_request",
			Title:  "Bad Request",
			Detail: &detail,
		}
	case errors.Is(err, trip.ErrTripNotFound):
		detail := "Trip not found"
		return http.StatusNotFound, api.Problem{
			Status: http.StatusNotFound,
			Code:   "trip_not_found",
			Title:  "Not Found",
			Detail: &detail,
		}
	case errors.Is(err, trip.ErrTripCompleted):
		detail := "Trip is already completed"
		return http.StatusConflict, api.Problem{
			Status: http.StatusConflict,
			Code:   "trip_already_completed",
			Title:  "Conflict",
			Detail: &detail,
		}
	case errors.Is(err, trip.ErrDriverBusy):
		detail := "Driver already has an active trip"
		return http.StatusConflict, api.Problem{
			Status: http.StatusConflict,
			Code:   "driver_busy",
			Title:  "Conflict",
			Detail: &detail,
		}
	default:
		detail := "Internal server error"
		return http.StatusInternalServerError, api.Problem{
			Status: http.StatusInternalServerError,
			Code:   "internal_error",
			Title:  "Internal Error",
			Detail: &detail,
		}
	}
}

func WriteError(w http.ResponseWriter, err error) {
	status, prob := Problem(err)
	WriteProblem(w, status, prob)
}

func WriteProblem(w http.ResponseWriter, status int, prob api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}
