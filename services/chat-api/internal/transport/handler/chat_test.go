package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm"
)

type mockChatSvc struct {
	streamChatFn func(ctx context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error)
}

func (m *mockChatSvc) StreamChat(ctx context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error) {
	return m.streamChatFn(ctx, providerName, model, message, conversationID)
}

var _ ChatService = (*mockChatSvc)(nil)

func TestChatHandler_Post(t *testing.T) {
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Delta: "Hello"}
	ch <- llm.StreamEvent{Done: true}
	close(ch)

	svc := &mockChatSvc{
		streamChatFn: func(_ context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error) {
			return &service.StreamResult{
				Events:          ch,
				ConversationID:  "conv-1",
				UserMessageID:   "msg-1",
			}, nil
		},
	}
	h := NewChatHandler(svc, slog.Default())

	body := `{"provider":"openai","model":"gpt-4","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected no-cache, got %q", w.Header().Get("Cache-Control"))
	}

	bodyOut := w.Body.String()
	if !strings.Contains(bodyOut, "event: meta") {
		t.Error("expected event: meta in SSE output")
	}
	if !strings.Contains(bodyOut, "event: token") {
		t.Error("expected event: token in SSE output")
	}
	if !strings.Contains(bodyOut, "event: done") {
		t.Error("expected event: done in SSE output")
	}
}

func TestChatHandler_MethodNotAllowed(t *testing.T) {
	h := NewChatHandler(&mockChatSvc{}, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	h := NewChatHandler(&mockChatSvc{}, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandler_EmptyMessage(t *testing.T) {
	h := NewChatHandler(&mockChatSvc{}, slog.Default())
	body := `{"provider":"openai","model":"gpt-4","message":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandler_UnknownProvider(t *testing.T) {
	svc := &mockChatSvc{
		streamChatFn: func(_ context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error) {
			return nil, &service.InputError{Field: "provider", Message: "unsupported provider: bad"}
		},
	}
	h := NewChatHandler(svc, slog.Default())
	body := `{"provider":"bad","model":"gpt-4","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandler_ConversationNotFound(t *testing.T) {
	svc := &mockChatSvc{
		streamChatFn: func(_ context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error) {
			return nil, service.ErrConversationNotFound
		},
	}
	h := NewChatHandler(svc, slog.Default())
	body := `{"provider":"openai","model":"gpt-4","message":"hello","conversation_id":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestChatHandler_CancelledConversation(t *testing.T) {
	svc := &mockChatSvc{
		streamChatFn: func(_ context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error) {
			return nil, service.ErrConversationCancelled
		},
	}
	h := NewChatHandler(svc, slog.Default())
	body := `{"provider":"openai","model":"gpt-4","message":"hello","conversation_id":"cancelled-conv"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
