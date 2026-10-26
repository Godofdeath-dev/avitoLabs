package create

import (
	"encoding/json"
	"log/slog"
	"net/http"

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
	var body api.CreateTripJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		detail := "Invalid JSON payload"
		domainerror.WriteProblem(w, http.StatusBadRequest, api.Problem{
			Status: http.StatusBadRequest,
			Code:   "invalid_request",
			Title:  "Bad Request",
			Detail: &detail,
		})
		return
	}

	res, err := h.uc.Execute(r.Context(), toInput(body))
	if err != nil {
		status, prob := domainerror.Problem(err)
		if status >= http.StatusInternalServerError && h.logger != nil {
			h.logger.ErrorContext(r.Context(), "failed to create trip", slog.String("error", err.Error()))
		}
		domainerror.WriteProblem(w, status, prob)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toResponse(res))
}
