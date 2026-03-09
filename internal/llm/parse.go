package llm

import (
	"encoding/json"
	"fmt"
	"time"
)

// rawEvent mirrors the JSON schema returned by the LLM.
type rawEvent struct {
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description"`
	Location    string `json:"location"`
	AllDay      bool   `json:"all_day"`
}

// parseResponse unmarshals the LLM's JSON response body into a ParsedEvent.
func parseResponse(body string) (*ParsedEvent, error) {
	var raw rawEvent
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("parsing LLM response: unmarshal JSON: %w", err)
	}

	if raw.Title == "" {
		return nil, fmt.Errorf("parsing LLM response: title is required but was empty")
	}

	event := &ParsedEvent{
		Title:       raw.Title,
		Description: raw.Description,
		Location:    raw.Location,
		AllDay:      raw.AllDay,
	}

	if raw.AllDay {
		// Validate and store the YYYY-MM-DD strings; parsing confirms format correctness.
		if _, err := time.Parse("2006-01-02", raw.Start); err != nil {
			return nil, fmt.Errorf("parsing LLM response: invalid all-day start date %q: %w", raw.Start, err)
		}
		if _, err := time.Parse("2006-01-02", raw.End); err != nil {
			return nil, fmt.Errorf("parsing LLM response: invalid all-day end date %q: %w", raw.End, err)
		}
		event.StartDate = raw.Start
		event.EndDate = raw.End
		return event, nil
	}

	// Timed event: parse RFC3339 timestamps, preserving the timezone offset.
	if raw.Start == "" {
		return nil, fmt.Errorf("parsing LLM response: start time is required for timed events")
	}
	start, err := time.Parse(time.RFC3339, raw.Start)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: parse start time %q: %w", raw.Start, err)
	}
	event.Start = start

	if raw.End == "" {
		return nil, fmt.Errorf("parsing LLM response: end time is required for timed events")
	}
	end, err := time.Parse(time.RFC3339, raw.End)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: parse end time %q: %w", raw.End, err)
	}
	event.End = end

	return event, nil
}
