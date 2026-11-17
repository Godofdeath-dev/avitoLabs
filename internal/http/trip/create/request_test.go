package create_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/trip/create"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

func TestRequiredRequestFields(t *testing.T) {
	t.Parallel()
	const valid = `{"user_id":"00000000-0000-0000-0000-000000000001","driver_id":"00000000-0000-0000-0000-000000000002","price":0,"start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":0,"longitude":0}}`
	for _, body := range []string{"{}", "null", valid + " {}", strings.Replace(valid, `"latitude":0`, `"extra":0`, 1), strings.Replace(valid, `"price":0`, `"price":null`, 1), strings.Replace(valid, `"price":0`, `"price":0,"extra":1`, 1)} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			uc := NewMockusecase(gomock.NewController(t))
			response := httptest.NewRecorder()
			create.New(uc, nil).Handle(response, httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body)))
			var problem api.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if response.Code != 400 || problem.Code != "invalid_request" || problem.Type != "about:blank" {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
	t.Run("zero values are present and valid", func(t *testing.T) {
		uc := NewMockusecase(gomock.NewController(t))
		id := uuid.New()
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, in model.CreateTripInput) (*trip.Trip, error) {
			if in.Price != 0 || in.StartPoint.Latitude != 0 {
				t.Error("zero values changed")
			}
			return &trip.Trip{ID: id, Status: trip.StatusActive, StartedAt: time.Now()}, nil
		})
		response := httptest.NewRecorder()
		create.New(uc, nil).Handle(response, httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(valid)))
		if response.Code != 201 || response.Header().Get("Location") != "/api/v1/trips/"+id.String() {
			t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
		}
	})
}
