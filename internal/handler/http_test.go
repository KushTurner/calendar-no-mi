package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kushturner/calendar-no-mi/internal/config"
	"github.com/kushturner/calendar-no-mi/internal/models"
)

// stubService implements eventCreator for testing.
type stubService struct {
	result models.Result
	err    error
}

func (s *stubService) CreateFromText(_ context.Context, _ models.User, _ string) (models.Result, error) {
	return s.result, s.err
}

func testConfig() *config.Config {
	return &config.Config{
		HTTPBearerToken: "test-token",
		DefaultTimezone: "America/New_York",
		GoogleCalendarID: "primary",
	}
}

func TestHandler_CreateEvent(t *testing.T) {
	eventURL := "https://calendar.google.com/event/abc"

	tests := []struct {
		name           string
		authHeader     string
		body           string
		svcResult      models.Result
		svcErr         error
		wantStatus     int
		wantBodyKey    string
		wantBodyValue  string
	}{
		{
			name:        "missing authorization header",
			authHeader:  "",
			body:        `{"text":"lunch tomorrow"}`,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:        "wrong bearer token",
			authHeader:  "Bearer wrong-token",
			body:        `{"text":"lunch tomorrow"}`,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:        "empty text field",
			authHeader:  "Bearer test-token",
			body:        `{"text":""}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing text field",
			authHeader:  "Bearer test-token",
			body:        `{}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:       "service error returns 500 with json body",
			authHeader: "Bearer test-token",
			body:       `{"text":"lunch tomorrow"}`,
			svcErr:     errors.New("service: create event: calendar offline"),
			wantStatus: http.StatusInternalServerError,
			wantBodyKey:   "error",
			wantBodyValue: "internal server error",
		},
		{
			name:       "happy path returns 201 with event_url",
			authHeader: "Bearer test-token",
			body:       `{"text":"lunch tomorrow at noon"}`,
			svcResult:  models.Result{EventURL: eventURL},
			wantStatus: http.StatusCreated,
			wantBodyKey:   "event_url",
			wantBodyValue: eventURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(testConfig(), &stubService{result: tc.svcResult, err: tc.svcErr}, slog.Default())

			req := httptest.NewRequest(http.MethodPost, "/event", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rec := httptest.NewRecorder()
			h.CreateEvent(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantBodyKey != "" {
				var got map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode JSON body: %v (raw: %s)", err, rec.Body.String())
				}
				if got[tc.wantBodyKey] != tc.wantBodyValue {
					t.Errorf("body[%q] = %q, want %q", tc.wantBodyKey, got[tc.wantBodyKey], tc.wantBodyValue)
				}
			}
		})
	}
}
