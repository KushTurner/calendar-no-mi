package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kushturner/calendar-no-mi/internal/calendar"
	"github.com/kushturner/calendar-no-mi/internal/llm"
	"github.com/kushturner/calendar-no-mi/internal/models"
)

// EventService orchestrates LLM parsing and calendar creation.
type EventService struct {
	llm  llm.LLMProvider
	cal  calendar.CalendarClientFactory
	now  func() time.Time
}

// NewEventService constructs an EventService with the given LLM and calendar factory.
func NewEventService(llmProvider llm.LLMProvider, cal calendar.CalendarClientFactory) *EventService {
	return &EventService{llm: llmProvider, cal: cal, now: time.Now}
}

// CreateFromText parses natural-language text and creates a calendar event for the user.
func (s *EventService) CreateFromText(ctx context.Context, user models.User, text string) (models.Result, error) {
	// 1. Parse via LLM
	parsed, err := s.llm.ParseEvent(ctx, text, s.now(), user.Timezone)
	if err != nil {
		return models.Result{}, fmt.Errorf("service: parse event: %w", err)
	}

	// 2. Validate required time fields before any API calls.
	if parsed.AllDay && parsed.EndDate == "" {
		return models.Result{}, fmt.Errorf("service: parsed all-day event has no end date")
	}
	if !parsed.AllDay && parsed.End.IsZero() {
		return models.Result{}, fmt.Errorf("service: parsed event has no end time")
	}

	// 3. Map ParsedEvent → models.CalendarEvent
	var event models.CalendarEvent
	if parsed.AllDay {
		startDate, err := time.Parse("2006-01-02", parsed.StartDate)
		if err != nil {
			return models.Result{}, fmt.Errorf("service: parse start date: %w", err)
		}
		endDate, err := time.Parse("2006-01-02", parsed.EndDate)
		if err != nil {
			return models.Result{}, fmt.Errorf("service: parse end date: %w", err)
		}
		event = models.CalendarEvent{
			Title:       parsed.Title,
			Start:       startDate,
			End:         endDate,
			Description: parsed.Description,
			Location:    parsed.Location,
			AllDay:      true,
		}
	} else {
		event = models.CalendarEvent{
			Title:       parsed.Title,
			Start:       parsed.Start,
			End:         parsed.End,
			Description: parsed.Description,
			Location:    parsed.Location,
		}
	}

	// 4. Get calendar client for user
	calClient, err := s.cal.ForUser(ctx, user)
	if err != nil {
		return models.Result{}, fmt.Errorf("service: get calendar client: %w", err)
	}

	// 5. Create event
	url, err := calClient.CreateEvent(ctx, event)
	if err != nil {
		return models.Result{}, fmt.Errorf("service: create event: %w", err)
	}

	return models.Result{Event: &event, EventURL: url}, nil
}
