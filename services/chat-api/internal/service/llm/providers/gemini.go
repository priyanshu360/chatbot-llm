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

type GeminiProvider struct {
	apiKey  string
	baseURL string
}

func NewGemini(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		contents := toGeminiContents(req.Messages)
		body := map[string]interface{}{
			"contents": contents,
		}

		payload, _ := json.Marshal(body)
		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, req.Model, p.apiKey)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
		if err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("request creation failed: %w", err)}
			return
		}
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
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
				UsageMetadata *struct {
					PromptTokenCount     int `json:"promptTokenCount"`
					CandidatesTokenCount int `json:"candidatesTokenCount"`
				} `json:"usageMetadata"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			for _, c := range event.Candidates {
				for _, p := range c.Content.Parts {
					if p.Text != "" {
						ch <- StreamEvent{Delta: p.Text}
					}
				}
				if c.FinishReason != "" {
					inputTokens := 0
					outputTokens := 0
					if event.UsageMetadata != nil {
						inputTokens = event.UsageMetadata.PromptTokenCount
						outputTokens = event.UsageMetadata.CandidatesTokenCount
					}
					ch <- StreamEvent{
						Finish: true,
						Usage:  &TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens},
					}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err)}
		}
	}()

	return ch, nil
}

func toGeminiContents(msgs []ChatMessage) []map[string]interface{} {
	var contents []map[string]interface{}
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]string{
				{"text": m.Content},
			},
		})
	}
	return contents
}
