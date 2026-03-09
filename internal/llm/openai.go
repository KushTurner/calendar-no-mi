package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openAIProvider struct {
	client openai.Client
}

func newOpenAIProvider(apiKey string) *openAIProvider {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &openAIProvider{client: client}
}

// ParseEvent sends the user's natural-language text to OpenAI and returns a
// structured ParsedEvent. Temperature is fixed at 0 for deterministic output.
func (p *openAIProvider) ParseEvent(ctx context.Context, userText string, now time.Time, timezone string) (*ParsedEvent, error) {
	systemPrompt := BuildSystemPrompt(now, timezone)

	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userText),
		},
		Temperature: openai.Float(0),
		MaxTokens:   openai.Int(512),
	})
	if err != nil {
		return nil, fmt.Errorf("llm: openai request: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm: openai returned no choices")
	}

	content := resp.Choices[0].Message.Content
	parsed, err := parseResponse(content)
	if err != nil {
		return nil, fmt.Errorf("llm: parse response: %w", err)
	}
	return parsed, nil
}
