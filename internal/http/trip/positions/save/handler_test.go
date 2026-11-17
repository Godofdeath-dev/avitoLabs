package save_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/trip/positions/save"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

const validBody = `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-11-17T12:00:00Z"}`

func TestSavePositionHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"success", nil, 201, ""}, {"missing", trip.ErrTripNotFound, 404, "trip_not_found"}, {"completed", trip.ErrPositionTripCompleted, 409, "trip_completed"}, {"range", &trip.ValidationError{Field: "coordinates", Reason: "out of range"}, 400, "invalid_request"}, {"internal", errors.New("secret database password"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := NewMockusecase(gomock.NewController(t))
			id := uuid.New()
			uc.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, in model.SavePositionInput) (*trip.Position, error) {
				require.Equal(t, id, in.TripID)
				require.Equal(t, trip.Coordinates{}, in.Coordinates)
				return &trip.Position{ID: 42, TripID: id, RecordedAt: time.Now()}, tc.err
			})
			router := chi.NewRouter()
			router.Post("/trips/{tripId}/positions", save.New(uc, slog.New(slog.NewTextHandler(io.Discard, nil))).Handle)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trips/"+id.String()+"/positions", strings.NewReader(validBody)))
			require.Equal(t, tc.status, rec.Code)
			require.NotContains(t, rec.Body.String(), "secret database")
			if tc.err != nil {
				require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
				var body api.Problem
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, tc.code, body.Code)
			} else {
				var body api.TripPosition
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, int64(42), body.Id)
				require.Equal(t, id, body.TripId)
			}
		})
	}
}

func TestInvalidPositionPayload(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"{}", "null", validBody + " {}", strings.Replace(validBody, `"longitude":0`, `"longitude":null`, 1), strings.Replace(validBody, "2026-11-17T12:00:00Z", "not-a-date", 1), strings.Replace(validBody, `"latitude":0`, `"latitude":0,"extra":0`, 1)} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			uc := NewMockusecase(gomock.NewController(t))
			router := chi.NewRouter()
			router.Post("/trips/{tripId}/positions", save.New(uc, nil).Handle)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trips/"+uuid.NewString()+"/positions", strings.NewReader(body)))
			require.Equal(t, 400, rec.Code)
		})
	}
}
