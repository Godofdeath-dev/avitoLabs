package list

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/domainerror"
)

type Handler struct {
	uc     usecase
	logger *slog.Logger
}

func New(uc usecase, l *slog.Logger) *Handler { return &Handler{uc: uc, logger: l} }
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tripId"))
	if err != nil {
		domainerror.WriteError(w, &trip.ValidationError{Field: "tripId", Reason: "invalid UUID"})
		return
	}
	items, err := h.uc.Execute(r.Context(), id)
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= 500 {
			h.logger.ErrorContext(r.Context(), "list positions failed", "error", err)
		}
		domainerror.WriteProblem(w, status, problem)
		return
	}
	positions := make([]api.TripPosition, 0, len(items))
	for _, p := range items {
		positions = append(positions, toResponse(p))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.TripPositionsResponse{Positions: positions})
}
