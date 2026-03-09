package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/kushturner/calendar-no-mi/internal/models"
)

// buildFakeService creates a *gcal.Service pointed at the given test server.
func buildFakeService(t *testing.T, srv *httptest.Server) *gcal.Service {
	t.Helper()
	svc, err := gcal.NewService(context.Background(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL),
		option.WithoutAuthentication(),
	)
	if err != nil {
		t.Fatalf("buildFakeService: %v", err)
	}
	return svc
}

// writeJSON is a helper to write JSON responses in fake handlers.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// gcalTimedEvent returns a minimal timed Google Calendar API event.
func gcalTimedEvent(id, title, startDT, endDT string) map[string]any {
	return map[string]any{
		"kind":    "calendar#event",
		"id":      id,
		"summary": title,
		"start":   map[string]string{"dateTime": startDT},
		"end":     map[string]string{"dateTime": endDT},
	}
}

// eventsListPath matches the Google Calendar events list/get path for "primary".
func eventsListPath() string { return "/calendars/primary/events" }
func eventPath(id string) string {
	return "/calendars/primary/events/" + id
}

func TestListEvents_ParsesEventsCorrectly(t *testing.T) {
	start := time.Date(2026, 3, 8, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == eventsListPath() {
			writeJSON(w, map[string]any{
				"kind": "calendar#events",
				"items": []any{
					gcalTimedEvent("evt1", "Team Standup",
						start.Format(time.RFC3339), end.Format(time.RFC3339)),
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	events, err := client.ListEvents(context.Background(), start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "Team Standup" {
		t.Errorf("title: got %q, want %q", events[0].Title, "Team Standup")
	}
	if !events[0].Start.Equal(start) {
		t.Errorf("start: got %v, want %v", events[0].Start, start)
	}
}

func TestListEvents_PropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]any{"error": map[string]any{"code": 500, "message": "internal error"}})
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	_, err := client.ListEvents(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "calendar: list events") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestCreateEvent_SendsPayloadAndReturnsURL(t *testing.T) {
	wantTitle := "Product Review"
	start := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)
	wantURL := "https://calendar.google.com/event?eid=abc123"

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == eventsListPath() {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{
				"kind":     "calendar#event",
				"id":       "abc123",
				"summary":  wantTitle,
				"htmlLink": wantURL,
				"start":    map[string]string{"dateTime": start.Format(time.RFC3339)},
				"end":      map[string]string{"dateTime": end.Format(time.RFC3339)},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	gotURL, err := client.CreateEvent(context.Background(), models.CalendarEvent{
		Title: wantTitle,
		Start: start,
		End:   end,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if gotURL != wantURL {
		t.Errorf("URL: got %q, want %q", gotURL, wantURL)
	}
	if gotBody["summary"] != wantTitle {
		t.Errorf("payload summary: got %v, want %q", gotBody["summary"], wantTitle)
	}
}

func TestUpdateEvent_SendsCorrectPayload(t *testing.T) {
	eventID := "evt42"
	wantTitle := "Updated Meeting"
	start := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 11, 11, 0, 0, 0, time.UTC)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == eventPath(eventID) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{
				"kind":    "calendar#event",
				"id":      eventID,
				"summary": wantTitle,
				"start":   map[string]string{"dateTime": start.Format(time.RFC3339)},
				"end":     map[string]string{"dateTime": end.Format(time.RFC3339)},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	err := client.UpdateEvent(context.Background(), eventID, models.CalendarEvent{
		Title: wantTitle,
		Start: start,
		End:   end,
	})
	if err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}
	if gotBody["summary"] != wantTitle {
		t.Errorf("payload summary: got %v, want %q", gotBody["summary"], wantTitle)
	}
}

func TestUpdateEvent_PropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"error": map[string]any{"code": 404, "message": "not found"}})
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	err := client.UpdateEvent(context.Background(), "missing", models.CalendarEvent{
		Start: time.Now(),
		End:   time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "calendar: update event") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestGetEvent_ReturnsCorrectlyParsedEvent(t *testing.T) {
	eventID := "evt99"
	wantTitle := "Design Review"
	start := time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 12, 16, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == eventPath(eventID) {
			writeJSON(w, gcalTimedEvent(eventID, wantTitle,
				start.Format(time.RFC3339), end.Format(time.RFC3339)))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	event, err := client.GetEvent(context.Background(), eventID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if event.ID != eventID {
		t.Errorf("ID: got %q, want %q", event.ID, eventID)
	}
	if event.Title != wantTitle {
		t.Errorf("title: got %q, want %q", event.Title, wantTitle)
	}
	if !event.Start.Equal(start) {
		t.Errorf("start: got %v, want %v", event.Start, start)
	}
}

func TestGetEvent_PropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"error": map[string]any{"code": 404, "message": "not found"}})
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	_, err := client.GetEvent(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "calendar: get event") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestAllDayEvent_ParsedCorrectly(t *testing.T) {
	// All-day events use "date" instead of "dateTime".
	// Google Calendar's end date is exclusive; we expect the domain model to use inclusive.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == eventsListPath() {
			writeJSON(w, map[string]any{
				"kind": "calendar#events",
				"items": []any{
					map[string]any{
						"kind":    "calendar#event",
						"id":      "allday1",
						"summary": "Company Holiday",
						// Google sends exclusive end: event is March 15 only,
						// stored as start=Mar15, end=Mar16.
						"start": map[string]string{"date": "2026-03-15"},
						"end":   map[string]string{"date": "2026-03-16"},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &googleCalendarClient{svc: buildFakeService(t, srv), calendarID: "primary"}

	events, err := client.ListEvents(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if !e.AllDay {
		t.Error("expected AllDay=true")
	}
	if e.Title != "Company Holiday" {
		t.Errorf("title: got %q, want %q", e.Title, "Company Holiday")
	}

	wantStart := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC) // inclusive: same day
	if !e.Start.Equal(wantStart) {
		t.Errorf("start: got %v, want %v", e.Start, wantStart)
	}
	if !e.End.Equal(wantEnd) {
		t.Errorf("end: got %v, want %v (should be inclusive, not Google's exclusive Mar 16)", e.End, wantEnd)
	}
}

func TestAllDayEvent_RoundTripConvention(t *testing.T) {
	// Verify that a multi-day all-day event round-trips correctly through
	// toGCalEvent → fromGCalEvent with the inclusive end-date convention.
	//
	// Domain: start=Jul 4, end=Jul 6 (inclusive, 3-day event)
	// → Google: start.date="2026-07-04", end.date="2026-07-07" (exclusive +1 day)
	// → Domain again: start=Jul 4, end=Jul 6 (inclusive, -1 day)
	domainStart := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	domainEnd := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC) // inclusive

	input := models.CalendarEvent{
		Title:  "Summer Break",
		Start:  domainStart,
		End:    domainEnd,
		AllDay: true,
	}

	gcalEvent := toGCalEvent(input)

	// Google exclusive end should be Jul 7.
	if gcalEvent.End.Date != "2026-07-07" {
		t.Errorf("gcal end.date: got %q, want %q", gcalEvent.End.Date, "2026-07-07")
	}

	// Round-trip back through fromGCalEvent.
	gcalItem := &gcal.Event{
		Id:      "rt1",
		Summary: input.Title,
		Start:   gcalEvent.Start,
		End:     gcalEvent.End,
	}
	result, err := fromGCalEvent(gcalItem)
	if err != nil {
		t.Fatalf("fromGCalEvent: %v", err)
	}
	if !result.Start.Equal(domainStart) {
		t.Errorf("round-trip start: got %v, want %v", result.Start, domainStart)
	}
	if !result.End.Equal(domainEnd) {
		t.Errorf("round-trip end: got %v, want %v (should be inclusive Jul 6, not Google's exclusive Jul 7)", result.End, domainEnd)
	}
}
