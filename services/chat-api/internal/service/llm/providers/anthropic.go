package providers

import (
	"context"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client *anthropic.Client
}

func NewAnthropic(apiKey string) *AnthropicProvider {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: &c}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		var systemContent string
		var msgs []anthropic.MessageParam
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemContent = m.Content
				continue
			}
			if m.Role == "assistant" {
				msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
			} else {
				msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
			}
		}

		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(req.Model),
			MaxTokens: 4096,
			Messages:  msgs,
		}
		if systemContent != "" {
			params.System = []anthropic.TextBlockParam{{Text: systemContent}}
		}

		stream := p.client.Messages.NewStreaming(ctx, params)

		var msg anthropic.Message
		for stream.Next() {
			event := stream.Current()
			if err := msg.Accumulate(event); err != nil {
				ch <- StreamEvent{Error: err}
				return
			}
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				ch <- StreamEvent{Delta: event.Delta.Text}
			}
		}

		if err := stream.Err(); err != nil {
			var apiErr *anthropic.Error
			if errors.As(err, &apiErr) {
				ch <- StreamEvent{Error: &APIError{StatusCode: apiErr.StatusCode, Detail: apiErr.Error()}}
			} else {
				ch <- StreamEvent{Error: err}
			}
			return
		}

		ch <- StreamEvent{
			Finish: true,
			Usage: &TokenUsage{
				InputTokens:  int(msg.Usage.InputTokens),
				OutputTokens: int(msg.Usage.OutputTokens),
			},
		}
	}()

	return ch, nil
}
