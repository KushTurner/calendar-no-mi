package llm

import (
	"testing"
	"time"
)

func TestParseResponse(t *testing.T) {
	t.Run("timed event: RFC3339 with offset parses correctly", func(t *testing.T) {
		body := `{
			"title": "Team standup",
			"start": "2026-03-09T09:00:00-05:00",
			"end":   "2026-03-09T09:30:00-05:00",
			"description": "Daily sync",
			"location": "Zoom",
			"all_day": false
		}`

		got, err := parseResponse(body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Title != "Team standup" {
			t.Errorf("Title: want %q, got %q", "Team standup", got.Title)
		}
		if got.AllDay {
			t.Error("AllDay: want false, got true")
		}
		if got.Start.IsZero() {
			t.Error("Start: want non-zero, got zero")
		}
		if got.End.IsZero() {
			t.Error("End: want non-zero, got zero")
		}

		// Verify timezone offset is preserved (-05:00).
		_, offsetSecs := got.Start.Zone()
		wantOffsetSecs := -5 * 60 * 60
		if offsetSecs != wantOffsetSecs {
			t.Errorf("Start timezone offset: want %d seconds, got %d seconds", wantOffsetSecs, offsetSecs)
		}

		wantStart := time.Date(2026, 3, 9, 9, 0, 0, 0, time.FixedZone("", -5*60*60))
		if !got.Start.Equal(wantStart) {
			t.Errorf("Start: want %v, got %v", wantStart, got.Start)
		}

		if got.Description != "Daily sync" {
			t.Errorf("Description: want %q, got %q", "Daily sync", got.Description)
		}
		if got.Location != "Zoom" {
			t.Errorf("Location: want %q, got %q", "Zoom", got.Location)
		}
		// StartDate/EndDate should be empty for timed events.
		if got.StartDate != "" {
			t.Errorf("StartDate: want empty, got %q", got.StartDate)
		}
		if got.EndDate != "" {
			t.Errorf("EndDate: want empty, got %q", got.EndDate)
		}
	})

	t.Run("all-day event: StartDate and EndDate populated, time.Time zero", func(t *testing.T) {
		body := `{
			"title": "Company holiday",
			"start": "2026-07-04",
			"end":   "2026-07-04",
			"description": "",
			"location": "",
			"all_day": true
		}`

		got, err := parseResponse(body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Title != "Company holiday" {
			t.Errorf("Title: want %q, got %q", "Company holiday", got.Title)
		}
		if !got.AllDay {
			t.Error("AllDay: want true, got false")
		}
		if got.StartDate != "2026-07-04" {
			t.Errorf("StartDate: want %q, got %q", "2026-07-04", got.StartDate)
		}
		if got.EndDate != "2026-07-04" {
			t.Errorf("EndDate: want %q, got %q", "2026-07-04", got.EndDate)
		}
		if !got.Start.IsZero() {
			t.Errorf("Start: want zero time, got %v", got.Start)
		}
		if !got.End.IsZero() {
			t.Errorf("End: want zero time, got %v", got.End)
		}
	})

	t.Run("error: empty title returns error", func(t *testing.T) {
		body := `{
			"title": "",
			"start": "2026-03-09T09:00:00-05:00",
			"end":   "2026-03-09T10:00:00-05:00",
			"all_day": false
		}`

		_, err := parseResponse(body)
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	t.Run("error: invalid end time for timed event returns error", func(t *testing.T) {
		body := `{
			"title": "Meeting",
			"start": "2026-03-09T09:00:00-05:00",
			"end":   "not-a-date",
			"all_day": false
		}`

		_, err := parseResponse(body)
		if err == nil {
			t.Fatal("expected error for invalid end time, got nil")
		}
	})

	t.Run("error: invalid JSON returns error", func(t *testing.T) {
		body := `{ this is not valid JSON`

		_, err := parseResponse(body)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}
