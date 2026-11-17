package save

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/domainerror"
	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

const bodyField = "body"

type Handler struct {
	uc     usecase
	logger *slog.Logger
}

func New(uc usecase, l *slog.Logger) *Handler { return &Handler{uc: uc, logger: l} }

// Указатели позволяют отличить пропущенное поле от допустимого значения 0.
type coordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}
type body struct {
	Coordinates *coordinates `json:"coordinates"`
	RecordedAt  *time.Time   `json:"recorded_at"`
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tripId"))
	if err != nil {
		domainerror.WriteError(w, &trip.ValidationError{Field: "tripId", Reason: "invalid UUID"})
		return
	}
	var b body
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		domainerror.WriteError(w, &trip.ValidationError{Field: bodyField, Reason: "invalid JSON"})
		return
	}
	// После объекта допустим только конец тела, а не второй JSON-объект.
	if err := dec.Decode(new(any)); err != io.EOF {
		domainerror.WriteError(w, &trip.ValidationError{Field: bodyField, Reason: "expected one JSON object"})
		return
	}
	if b.Coordinates == nil || b.RecordedAt == nil || b.Coordinates.Latitude == nil || b.Coordinates.Longitude == nil {
		domainerror.WriteError(w, &trip.ValidationError{Field: bodyField, Reason: "required fields missing"})
		return
	}
	p, err := h.uc.Execute(r.Context(), model.SavePositionInput{TripID: id, Coordinates: trip.Coordinates{Latitude: *b.Coordinates.Latitude, Longitude: *b.Coordinates.Longitude}, RecordedAt: *b.RecordedAt})
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= 500 {
			h.logger.ErrorContext(r.Context(), "save position failed", "error", err)
		}
		domainerror.WriteProblem(w, status, problem)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toResponse(*p))
}
