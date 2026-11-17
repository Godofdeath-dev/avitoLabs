package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	db      pinger
	timeout time.Duration
}

func New(db pinger, timeout time.Duration) *Handler {
	return &Handler{db: db, timeout: timeout}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if h.db == nil || h.db.Ping(ctx) != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
