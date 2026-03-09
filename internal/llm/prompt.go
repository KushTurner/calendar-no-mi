package llm

import (
	"fmt"
	"time"
)

// BuildSystemPrompt returns the system prompt for the LLM, injecting the current
// datetime and user timezone so the model can resolve relative references like
// "tomorrow" or "next Friday".
func BuildSystemPrompt(now time.Time, timezone string) string {
	return fmt.Sprintf(`You are a calendar assistant. Parse the user's natural-language event description and return a JSON object describing the event.

Current datetime: %s
User's timezone: %s

Return JSON only — no prose, no markdown fences, no explanation. The JSON must match this schema exactly:

{
  "title":       string,   // required, event title
  "start":       string,   // RFC3339 with timezone offset (e.g. "2026-03-09T14:00:00-05:00"), or YYYY-MM-DD if all_day
  "end":         string,   // RFC3339 with timezone offset, or YYYY-MM-DD if all_day; if no end time given, default to 1 hour after start
  "description": string,   // optional, may be empty string
  "location":    string,   // optional, may be empty string
  "all_day":     boolean   // true if this is an all-day event with no specific time
}

Rules:
- start and end must use RFC3339 format with the timezone offset matching the user's timezone (%s)
- if no end time is specified, default end to exactly 1 hour after start
- for all-day events: set all_day to true, and use YYYY-MM-DD strings for start and end (no time component)
- return JSON only — no prose, no markdown fences`,
		now.Format(time.RFC3339),
		timezone,
		timezone,
	)
}
