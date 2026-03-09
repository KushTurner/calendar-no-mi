package models

import "time"

// User holds per-user state and credentials.
type User struct {
	ID           string
	RefreshToken string // never log — contains OAuth credential
	CalendarID   string
	Timezone     string
	RegisteredAt time.Time
}

// CalendarEvent is the domain representation of a calendar event.
type CalendarEvent struct {
	ID          string
	Title       string
	Start       time.Time
	End         time.Time
	Description string
	Location    string
	Attendees   []string
	AllDay      bool
}

// Result is the outcome returned to the caller after processing an event request.
type Result struct {
	Event    *CalendarEvent
	EventURL string
}
