package finish

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/domainerror"
)

type Handler struct {
	uc     usecase
	logger *slog.Logger
}

func New(uc usecase, l *slog.Logger) *Handler {
	return &Handler{uc: uc, logger: l}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tripId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		detail := "Invalid tripId format"
		domainerror.WriteProblem(w, http.StatusBadRequest, api.Problem{
			Status: http.StatusBadRequest,
			Code:   "invalid_request",
			Title:  "Bad Request",
			Detail: &detail,
		})
		return
	}

	res, err := h.uc.Execute(r.Context(), id)
	if err != nil {
		status, prob := domainerror.Problem(err)
		if status >= http.StatusInternalServerError && h.logger != nil {
			h.logger.ErrorContext(r.Context(), "failed to finish trip", slog.String("error", err.Error()))
		}
		domainerror.WriteProblem(w, status, prob)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toResponse(res))
}
