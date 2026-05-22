package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

func TestMessagesHandler_Get(t *testing.T) {
	svc := &mockConvSvc{
		msgsFn: func(_ context.Context, conversationID string) ([]pkg.Message, error) {
			return []pkg.Message{
				{Role: "user", Content: "hi", Seq: 1},
				{Role: "assistant", Content: "hello", Seq: 2},
			}, nil
		},
	}
	h := NewMessagesHandler(svc, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/abc/messages", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var msgs []pkg.Message
	if err := json.NewDecoder(w.Body).Decode(&msgs); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestMessagesHandler_MethodNotAllowed(t *testing.T) {
	h := NewMessagesHandler(&mockConvSvc{}, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/conversations/abc/messages", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
