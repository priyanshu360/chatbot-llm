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

type AnthropicProvider struct {
	apiKey  string
	baseURL string
}

func NewAnthropic(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: "https://api.anthropic.com/v1",
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		systemContent := ""
		var msgs []map[string]string
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemContent = m.Content
				continue
			}
			role := m.Role
			if role == "assistant" {
				role = "assistant"
			}
			msgs = append(msgs, map[string]string{"role": role, "content": m.Content})
		}

		body := map[string]interface{}{
			"model":      req.Model,
			"messages":   msgs,
			"stream":     true,
			"max_tokens": 4096,
		}
		if systemContent != "" {
			body["system"] = systemContent
		}

		payload, _ := json.Marshal(body)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(payload))
		if err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("request creation failed: %w", err)}
			return
		}

		httpReq.Header.Set("x-api-key", p.apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("api call failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			ch <- StreamEvent{Error: &APIError{StatusCode: resp.StatusCode, Detail: readErrorBody(resp)}}
			return
		}

		inputTokens := 0
		outputTokens := 0
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if strings.TrimSpace(data) == "" {
				continue
			}

			var event struct {
				Type string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
				Message struct {
					Usage *struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
				ContentBlock *struct {
					Text string `json:"text"`
				} `json:"content_block"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "message_start":
				if event.Message.Usage != nil {
					inputTokens = event.Message.Usage.InputTokens
				}
			case "content_block_delta":
				if event.Delta.Text != "" {
					ch <- StreamEvent{Delta: event.Delta.Text}
				}
			case "message_delta":
				if usage := event.Message.Usage; usage != nil {
					outputTokens = usage.OutputTokens
				}
			case "message_stop":
				ch <- StreamEvent{
					Finish: true,
					Usage:  &TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens},
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err)}
		}
	}()

	return ch, nil
}
