package list_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/trip/positions/list"
)

func TestListPositionHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		empty  bool
	}{{"route", nil, 200, false}, {"empty", nil, 200, true}, {"missing", trip.ErrTripNotFound, 404, false}, {"internal", errors.New("secret SQL"), 500, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := NewMockusecase(gomock.NewController(t))
			id := uuid.New()
			var points []trip.Position
			if !tc.empty {
				points = []trip.Position{{ID: 1, TripID: id, RecordedAt: time.Now()}}
			}
			uc.EXPECT().Execute(gomock.Any(), id).Return(points, tc.err)
			router := chi.NewRouter()
			router.Get("/trips/{tripId}/positions", list.New(uc, slog.New(slog.NewTextHandler(io.Discard, nil))).Handle)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trips/"+id.String()+"/positions", nil))
			require.Equal(t, tc.status, rec.Code)
			require.NotContains(t, rec.Body.String(), "secret SQL")
			if tc.err == nil {
				var body api.TripPositionsResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				if tc.empty {
					require.JSONEq(t, `{"positions":[]}`, rec.Body.String())
				} else {
					require.Len(t, body.Positions, 1)
					require.Equal(t, id, body.Positions[0].TripId)
				}
			}
		})
	}
}
