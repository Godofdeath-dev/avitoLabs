package domainerror_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "validation error -> 400 invalid_request",
			err:        &trip.ValidationError{Field: "price", Reason: "must be positive"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "not found -> 404 trip_not_found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "already completed -> 409 trip_already_completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_already_completed",
		},
		{
			name:       "driver busy -> 409 driver_busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "unknown db error -> 500 internal_error without leaking details",
			err:        errors.New("pq: password authentication failed for user postgres"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, prob := domainerror.Problem(tt.err)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if prob.Code != tt.wantCode {
				t.Errorf("code = %s, want %s", prob.Code, tt.wantCode)
			}
			// Сценарий 7: проверяем, что детали ошибки БД не утекли наружу
			if status == http.StatusInternalServerError && prob.Detail != nil {
				if strings.Contains(*prob.Detail, "postgres") {
					t.Errorf("internal error leaked sensitive detail: %s", *prob.Detail)
				}
			}
		})
	}
}
