package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) Routes(r chi.Router) {
	r.Get("/health", h.Health)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}
