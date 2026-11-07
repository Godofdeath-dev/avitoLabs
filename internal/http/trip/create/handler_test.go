package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/trip/create"
)

func TestHandler_Handle(t *testing.T) {
	t.Parallel()

	// Сценарий 6: невалидное тело запроса -> 400 с кодом invalid_request
	t.Run("invalid json returns 400 invalid_request", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := create.New(uc, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString("{broken json"))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		var prob api.Problem
		_ = json.NewDecoder(rec.Body).Decode(&prob)
		if prob.Code != "invalid_request" {
			t.Errorf("code = %s, want invalid_request", prob.Code)
		}
	})

	// Сценарий 2: занятый водитель -> 409
	t.Run("driver busy returns 409", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, trip.ErrDriverBusy)

		h := create.New(uc, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString(`{"price":1000}`))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
	})

	// Сценарий 7: ошибка репозитория -> 500 и отсутствие деталей ошибки в теле ответа
	t.Run("repository error returns 500 without details", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, errors.New("SELECT * FROM secret_table failed"))

		h := create.New(uc, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString(`{"price":1000}`))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "secret_table") {
			t.Errorf("response body leaked internal error: %s", rec.Body.String())
		}
	})
}
