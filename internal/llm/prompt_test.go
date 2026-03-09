package llm

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSystemPrompt(t *testing.T) {
	now := time.Date(2026, 3, 9, 14, 0, 0, 0, time.FixedZone("EST", -5*60*60))
	timezone := "America/New_York"

	prompt := BuildSystemPrompt(now, timezone)

	t.Run("includes injected datetime", func(t *testing.T) {
		datetime := now.Format(time.RFC3339)
		if !strings.Contains(prompt, datetime) {
			t.Errorf("expected prompt to contain datetime %q, got:\n%s", datetime, prompt)
		}
	})

	t.Run("includes timezone string", func(t *testing.T) {
		if !strings.Contains(prompt, timezone) {
			t.Errorf("expected prompt to contain timezone %q, got:\n%s", timezone, prompt)
		}
	})

	t.Run("instructs JSON output", func(t *testing.T) {
		if !strings.Contains(prompt, "JSON") {
			t.Errorf("expected prompt to contain the word 'JSON', got:\n%s", prompt)
		}
	})
}
