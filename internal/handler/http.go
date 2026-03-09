package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/kushturner/calendar-no-mi/internal/config"
	"github.com/kushturner/calendar-no-mi/internal/models"
)

// eventCreator is the subset of service.EventService used by Handler, defined here for testability.
type eventCreator interface {
	CreateFromText(ctx context.Context, user models.User, text string) (models.Result, error)
}

// Handler holds HTTP route handlers. Routes are registered in main.go.
type Handler struct {
	cfg     *config.Config
	service eventCreator
}

// NewHandler constructs a Handler. svc must implement eventCreator (service.EventService does).
func NewHandler(cfg *config.Config, svc eventCreator) *Handler {
	return &Handler{cfg: cfg, service: svc}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// CreateEvent handles POST /event.
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	// Belt-and-suspenders: misconfigured server should never accept requests.
	if h.cfg.HTTPBearerToken == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Auth: timing-safe bearer token comparison.
	authHeader := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.HTTPBearerToken)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse body.
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	// Build single-user from config.
	user := models.User{
		ID:           "default",
		Timezone:     h.cfg.DefaultTimezone,
		RefreshToken: h.cfg.GoogleRefreshToken,
		CalendarID:   h.cfg.GoogleCalendarID,
	}

	result, err := h.service.CreateFromText(r.Context(), user, body.Text)
	if err != nil {
		log.Printf("CreateEvent error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if result.EventURL == "" {
		log.Printf("CreateEvent: service returned empty event URL")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"event_url": result.EventURL})
}
