package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kushturner/calendar-no-mi/internal/calendar"
	"github.com/kushturner/calendar-no-mi/internal/llm"
	"github.com/kushturner/calendar-no-mi/internal/models"
)

// --- Stubs ---

type stubLLM struct {
	result *llm.ParsedEvent
	err    error
}

func (s *stubLLM) ParseEvent(_ context.Context, _ string, _ time.Time, _ string) (*llm.ParsedEvent, error) {
	return s.result, s.err
}

type stubCalClient struct {
	url string
	err error
}

func (c *stubCalClient) ListEvents(_ context.Context, _, _ time.Time) ([]models.CalendarEvent, error) {
	return nil, nil
}

func (c *stubCalClient) CreateEvent(_ context.Context, _ models.CalendarEvent) (string, error) {
	return c.url, c.err
}

func (c *stubCalClient) UpdateEvent(_ context.Context, _ string, _ models.CalendarEvent) error {
	return nil
}

func (c *stubCalClient) GetEvent(_ context.Context, _ string) (models.CalendarEvent, error) {
	return models.CalendarEvent{}, nil
}

// Verify stubCalClient satisfies calendar.CalendarClient at compile time.
var _ calendar.CalendarClient = (*stubCalClient)(nil)

type stubCalFactory struct {
	client *stubCalClient
	err    error
}

func (f *stubCalFactory) ForUser(_ context.Context, _ models.User) (calendar.CalendarClient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

// Verify stubCalFactory satisfies calendar.CalendarClientFactory at compile time.
var _ calendar.CalendarClientFactory = (*stubCalFactory)(nil)

// --- Tests ---

func TestEventService_CreateFromText(t *testing.T) {
	now := time.Now()
	end := now.Add(time.Hour)

	tests := []struct {
		name       string
		llmResult  *llm.ParsedEvent
		llmErr     error
		calURL     string
		calErr     error
		factoryErr error
		wantErrStr string
		wantURL    string
	}{
		{
			name: "happy path timed event",
			llmResult: &llm.ParsedEvent{
				Title: "Team Sync",
				Start: now,
				End:   end,
			},
			calURL:  "https://calendar.google.com/event/123",
			wantURL: "https://calendar.google.com/event/123",
		},
		{
			name: "happy path all-day event",
			llmResult: &llm.ParsedEvent{
				Title:     "Company Holiday",
				AllDay:    true,
				StartDate: "2026-03-10",
				EndDate:   "2026-03-10",
			},
			calURL:  "https://calendar.google.com/event/456",
			wantURL: "https://calendar.google.com/event/456",
		},
		{
			name:       "llm error propagates",
			llmErr:     errors.New("openai: rate limit"),
			wantErrStr: "service: parse event",
		},
		{
			name: "zero end time validation",
			llmResult: &llm.ParsedEvent{
				Title: "Missing End",
				Start: now,
				// End is zero value — should trigger validation error
			},
			wantErrStr: "service: parsed event has no end time",
		},
		{
			name: "calendar create error propagates",
			llmResult: &llm.ParsedEvent{
				Title: "Meeting",
				Start: now,
				End:   end,
			},
			calErr:     errors.New("google: quota exceeded"),
			wantErrStr: "service: create event",
		},
		{
			name: "factory error propagates",
			llmResult: &llm.ParsedEvent{
				Title: "Meeting",
				Start: now,
				End:   end,
			},
			factoryErr: errors.New("oauth: invalid token"),
			wantErrStr: "service: get calendar client",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewEventService(
				&stubLLM{result: tc.llmResult, err: tc.llmErr},
				&stubCalFactory{
					client: &stubCalClient{url: tc.calURL, err: tc.calErr},
					err:    tc.factoryErr,
				},
			)

			user := models.User{
				ID:       "u1",
				Timezone: "America/New_York",
			}

			result, err := svc.CreateFromText(context.Background(), user, "test input")

			if tc.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrStr)
				}
				if !strings.Contains(err.Error(), tc.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrStr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.EventURL != tc.wantURL {
				t.Errorf("EventURL = %q, want %q", result.EventURL, tc.wantURL)
			}

			if result.Event == nil {
				t.Fatal("expected non-nil Event in result")
			}
		})
	}
}
