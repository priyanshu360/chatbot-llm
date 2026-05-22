package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAIProvider struct {
	apiKey  string
	baseURL string
}

func NewOpenAI(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		body := map[string]interface{}{
			"model":    req.Model,
			"messages": toOpenAIMessages(req.Messages),
			"stream":   true,
		}

		payload, _ := json.Marshal(body)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("request creation failed: %w", err)}
			return
		}

		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("api call failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			ch <- StreamEvent{Error: fmt.Errorf("api error: %s", readErrorBody(resp))}
			return
		}

		totalInput := 0
		totalOutput := 0
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamEvent{
					Finish: true,
					Usage:  &TokenUsage{InputTokens: totalInput, OutputTokens: totalOutput},
				}
				return
			}

			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Usage != nil {
				totalInput = event.Usage.PromptTokens
				totalOutput = event.Usage.CompletionTokens
			}

			for _, c := range event.Choices {
				if c.FinishReason != nil {
					ch <- StreamEvent{
						Finish: true,
						Usage:  &TokenUsage{InputTokens: totalInput, OutputTokens: totalOutput},
					}
					return
				}
				if c.Delta.Content != "" {
					ch <- StreamEvent{Delta: c.Delta.Content}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err)}
		}
	}()

	return ch, nil
}

func toOpenAIMessages(msgs []ChatMessage) []map[string]string {
	result := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		result[i] = map[string]string{"role": role, "content": m.Content}
	}
	return result
}
