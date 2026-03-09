package llm

import (
	"context"
	"time"
)

// LLMProvider parses natural-language text into structured calendar events.
type LLMProvider interface {
	ParseEvent(ctx context.Context, userText string, now time.Time, timezone string) (*ParsedEvent, error)
}
