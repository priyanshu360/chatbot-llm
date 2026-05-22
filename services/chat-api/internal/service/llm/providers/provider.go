package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
) 

func readErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil || len(body) == 0 {
		return fmt.Sprintf("status %d", resp.StatusCode)
	}
	return fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))
}

type StreamEvent struct {
	Delta      string
	Finish     bool
	Error      error
	Usage      *TokenUsage
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Messages []ChatMessage
	Model    string
}

type Provider interface {
	Name() string
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}
