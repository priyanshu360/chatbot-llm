package llm

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm/providers"
	"github.com/priyanshu360/chatbot-llm/pkg"
)

type LLMClient struct {
	provider   providers.Provider
	ingestLog  *Logger
	slog       *slog.Logger
}

func NewLLMClient(provider providers.Provider, ingestLog *Logger, slog *slog.Logger) *LLMClient {
	return &LLMClient{provider: provider, ingestLog: ingestLog, slog: slog}
}

type StreamResult struct {
	Events  <-chan StreamEvent
	RequestID string
}

type StreamEvent struct {
	Delta  string
	Done   bool
	Error  error
	Usage  *providers.TokenUsage
}

func uuid() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (c *LLMClient) StreamChat(ctx context.Context, req providers.ChatRequest, conversationID, sessionID string) *StreamResult {
	requestID := uuid()
	c.slog.Debug("stream chat start", "request_id", requestID, "conversation_id", conversationID, "model", req.Model)

	out := make(chan StreamEvent)

	providerCh, err := c.provider.StreamChat(ctx, req)
	if err != nil {
		c.slog.Debug("provider stream init failed", "request_id", requestID, "error", err)
		out <- StreamEvent{Error: err}
		close(out)
		return &StreamResult{Events: out, RequestID: requestID}
	}

	go func() {
		defer close(out)

		startedAt := time.Now()
		var lastInputPreview, lastOutputPreview string
		var inputTokens, outputTokens int
		var tokenCount int
		status := "success"

		if len(req.Messages) > 0 {
			lastInputPreview = RedactPII(truncate(req.Messages[len(req.Messages)-1].Content, 200))
		}

		for evt := range providerCh {
			if evt.Error != nil {
				status = "error"
				c.slog.Debug("provider stream error", "request_id", requestID, "error", evt.Error)
				out <- StreamEvent{Error: evt.Error}
				break
			}

			if evt.Delta != "" {
				tokenCount++
				lastOutputPreview += evt.Delta
				out <- StreamEvent{Delta: evt.Delta}
			}

			if evt.Finish {
				c.slog.Debug("provider stream finished", "request_id", requestID, "tokens", tokenCount)
				if evt.Usage != nil {
					inputTokens = evt.Usage.InputTokens
					outputTokens = evt.Usage.OutputTokens
				}
				out <- StreamEvent{Done: true}
			}
		}

		latencyMs := int(time.Since(startedAt).Milliseconds())
		c.slog.Debug("stream chat complete", "request_id", requestID, "latency_ms", latencyMs, "input_tokens", inputTokens, "output_tokens", outputTokens)

		c.ingestLog.Log(pkg.InferenceLog{
			RequestID:      requestID,
			SessionID:      sessionID,
			ConversationID: conversationID,
			Model:          req.Model,
			Provider:       c.provider.Name(),
			LatencyMs:      latencyMs,
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
			Status:         status,
			InputPreview:   lastInputPreview,
			OutputPreview:  RedactPII(truncate(lastOutputPreview, 200)),
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		})
	}()

	return &StreamResult{Events: out, RequestID: requestID}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
