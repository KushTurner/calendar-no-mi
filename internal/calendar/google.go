package calendar

import (
	"context"
	"fmt"
	"time"

	gcal "google.golang.org/api/calendar/v3"

	"github.com/kushturner/calendar-no-mi/internal/models"
)

// googleCalendarClient implements CalendarClient against the Google Calendar API.
type googleCalendarClient struct {
	svc        *gcal.Service
	calendarID string
}

// ListEvents returns all events in [start, end).
func (c *googleCalendarClient) ListEvents(ctx context.Context, start, end time.Time) ([]models.CalendarEvent, error) {
	resp, err := c.svc.Events.
		List(c.calendarID).
		Context(ctx).
		TimeMin(start.UTC().Format(time.RFC3339)).
		TimeMax(end.UTC().Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("calendar: list events: %w", err)
	}

	events := make([]models.CalendarEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		e, err := fromGCalEvent(item)
		if err != nil {
			return nil, fmt.Errorf("calendar: list events: parse item %q: %w", item.Id, err)
		}
		events = append(events, e)
	}
	return events, nil
}

// CreateEvent creates a new calendar event and returns its HTML link.
func (c *googleCalendarClient) CreateEvent(ctx context.Context, event models.CalendarEvent) (string, error) {
	gcalEvent := toGCalEvent(event)
	created, err := c.svc.Events.
		Insert(c.calendarID, gcalEvent).
		Context(ctx).
		SendUpdates("none").
		Do()
	if err != nil {
		return "", fmt.Errorf("calendar: create event: %w", err)
	}
	return created.HtmlLink, nil
}

// UpdateEvent replaces an existing event by ID.
func (c *googleCalendarClient) UpdateEvent(ctx context.Context, eventID string, event models.CalendarEvent) error {
	gcalEvent := toGCalEvent(event)
	_, err := c.svc.Events.
		Update(c.calendarID, eventID, gcalEvent).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("calendar: update event %s: %w", eventID, err)
	}
	return nil
}

// GetEvent retrieves a single event by ID.
func (c *googleCalendarClient) GetEvent(ctx context.Context, eventID string) (models.CalendarEvent, error) {
	item, err := c.svc.Events.
		Get(c.calendarID, eventID).
		Context(ctx).
		Do()
	if err != nil {
		return models.CalendarEvent{}, fmt.Errorf("calendar: get event %s: %w", eventID, err)
	}
	e, err := fromGCalEvent(item)
	if err != nil {
		return models.CalendarEvent{}, fmt.Errorf("calendar: get event %s: parse: %w", eventID, err)
	}
	return e, nil
}

// toGCalEvent converts a domain CalendarEvent into a Google Calendar Event.
//
// All-day event convention: CalendarEvent.End is inclusive (the last day of the event).
// Google Calendar uses an exclusive end date, so we add one day when encoding.
func toGCalEvent(e models.CalendarEvent) *gcal.Event {
	gcalEvent := &gcal.Event{
		Summary:     e.Title,
		Description: e.Description,
		Location:    e.Location,
	}

	if e.AllDay {
		// Google Calendar end date is exclusive; add one day to convert from inclusive.
		gcalEnd := e.End.AddDate(0, 0, 1)
		gcalEvent.Start = &gcal.EventDateTime{Date: e.Start.Format("2006-01-02")}
		gcalEvent.End = &gcal.EventDateTime{Date: gcalEnd.Format("2006-01-02")}
	} else {
		gcalEvent.Start = &gcal.EventDateTime{DateTime: e.Start.UTC().Format(time.RFC3339)}
		gcalEvent.End = &gcal.EventDateTime{DateTime: e.End.UTC().Format(time.RFC3339)}
	}

	for _, email := range e.Attendees {
		gcalEvent.Attendees = append(gcalEvent.Attendees, &gcal.EventAttendee{Email: email})
	}

	return gcalEvent
}

// fromGCalEvent converts a Google Calendar Event into a domain CalendarEvent.
//
// All-day event convention: CalendarEvent.End is inclusive (the last day of the event).
// Google Calendar uses an exclusive end date, so we subtract one day when decoding.
func fromGCalEvent(item *gcal.Event) (models.CalendarEvent, error) {
	e := models.CalendarEvent{
		ID:          item.Id,
		Title:       item.Summary,
		Description: item.Description,
		Location:    item.Location,
	}

	for _, a := range item.Attendees {
		e.Attendees = append(e.Attendees, a.Email)
	}

	// All-day events use Date; timed events use DateTime.
	if item.Start != nil {
		if item.Start.Date != "" {
			e.AllDay = true
			t, err := time.Parse("2006-01-02", item.Start.Date)
			if err != nil {
				return models.CalendarEvent{}, fmt.Errorf("parse start date %q: %w", item.Start.Date, err)
			}
			e.Start = t
		} else {
			t, err := time.Parse(time.RFC3339, item.Start.DateTime)
			if err != nil {
				return models.CalendarEvent{}, fmt.Errorf("parse start datetime %q: %w", item.Start.DateTime, err)
			}
			e.Start = t
		}
	}

	if item.End != nil {
		if item.End.Date != "" {
			t, err := time.Parse("2006-01-02", item.End.Date)
			if err != nil {
				return models.CalendarEvent{}, fmt.Errorf("parse end date %q: %w", item.End.Date, err)
			}
			// Google's end date is exclusive; subtract one day to make it inclusive.
			e.End = t.AddDate(0, 0, -1)
		} else {
			t, err := time.Parse(time.RFC3339, item.End.DateTime)
			if err != nil {
				return models.CalendarEvent{}, fmt.Errorf("parse end datetime %q: %w", item.End.DateTime, err)
			}
			e.End = t
		}
	}

	return e, nil
}
