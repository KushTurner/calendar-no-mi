package calendar

import (
	"context"
	"time"

	"github.com/kushturner/calendar-no-mi/internal/models"
)

// CalendarClient is the interface for interacting with a user's calendar.
type CalendarClient interface {
	// ListEvents returns all events in the given time range.
	ListEvents(ctx context.Context, start, end time.Time) ([]models.CalendarEvent, error)

	// CreateEvent creates a new event and returns its URL.
	CreateEvent(ctx context.Context, event models.CalendarEvent) (string, error)

	// UpdateEvent replaces an existing event identified by eventID.
	UpdateEvent(ctx context.Context, eventID string, event models.CalendarEvent) error

	// GetEvent returns a single event by ID.
	GetEvent(ctx context.Context, eventID string) (models.CalendarEvent, error)
}

// CalendarClientFactory builds a CalendarClient scoped to a specific user.
type CalendarClientFactory interface {
	ForUser(ctx context.Context, user models.User) (CalendarClient, error)
}
