package finish_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/trip/finish"
)

func TestHandlerResponses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"success", nil, 200, ""},
		{"missing", trip.ErrTripNotFound, 404, "trip_not_found"},
		{"completed", trip.ErrTripCompleted, 409, "trip_already_completed"},
		{"internal", errors.New("secret SQL connection error"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := NewMockusecase(gomock.NewController(t))
			id := uuid.New()
			result := &trip.Trip{ID: id, Status: trip.StatusActive, StartedAt: time.Now()}
			uc.EXPECT().Execute(gomock.Any(), id).Return(result, tc.err)
			router := chi.NewRouter()
			router.MethodFunc("POST", "/trips/{tripId}", finish.New(uc, nil).Handle)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("POST", "/trips/"+id.String(), nil))
			if response.Code != tc.status {
				t.Fatalf("status %d, want %d", response.Code, tc.status)
			}
			if tc.err != nil {
				var problem api.Problem
				if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != tc.code || problem.Type == "" {
					t.Fatalf("problem = %+v", problem)
				}
				if strings.Contains(response.Body.String(), "secret SQL") {
					t.Error("internal details leaked")
				}
			} else {
				var body api.Trip
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Id != id {
					t.Error("wrong trip returned")
				}
			}
		})
	}
	t.Run("invalid UUID", func(t *testing.T) {
		uc := NewMockusecase(gomock.NewController(t))
		router := chi.NewRouter()
		router.MethodFunc("POST", "/trips/{tripId}", finish.New(uc, nil).Handle)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/trips/invalid", nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
	})
}
