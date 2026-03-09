package llm

import (
	"fmt"

	"github.com/kushturner/calendar-no-mi/internal/config"
)

// NewFromConfig creates an LLMProvider from the given config (OpenAI only for v1).
func NewFromConfig(cfg *config.Config) (LLMProvider, error) {
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("llm: OPENAI_API_KEY is required")
	}
	return newOpenAIProvider(cfg.OpenAIAPIKey), nil
}
