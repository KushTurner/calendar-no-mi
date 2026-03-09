package llm

import "time"

// ParsedEvent is the result of parsing a user's natural-language event description.
type ParsedEvent struct {
	Title       string
	Start       time.Time // populated for timed events; RFC3339 with offset preserves tz
	End         time.Time // always set for timed events; never zero value
	Description string
	Location    string
	AllDay      bool
	StartDate   string // only used when AllDay=true, YYYY-MM-DD
	EndDate     string // only used when AllDay=true, YYYY-MM-DD
}
