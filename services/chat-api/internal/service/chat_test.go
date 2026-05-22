package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/repo"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm/providers"
)

type mockLLMClient struct {
	streamChatFn func(ctx context.Context, req providers.ChatRequest, conversationID, sessionID string) *llm.StreamResult
}

func (m *mockLLMClient) StreamChat(ctx context.Context, req providers.ChatRequest, conversationID, sessionID string) *llm.StreamResult {
	return m.streamChatFn(ctx, req, conversationID, sessionID)
}

var _ LLMClient = (*mockLLMClient)(nil)

func TestChatService_StreamChat_UnknownProvider(t *testing.T) {
	s := NewChatService(&mockConvRepo{}, &mockMsgRepo{}, map[string]LLMClient{}, slog.Default())
	_, err := s.StreamChat(context.Background(), "nonexistent", "gpt-4", "hello", "")
	var inputErr *InputError
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected InputError, got %T: %v", err, err)
	}
	if inputErr.Field != "provider" {
		t.Errorf("expected field 'provider', got %q", inputErr.Field)
	}
}

func TestChatService_StreamChat_EmptyMessage(t *testing.T) {
	s := NewChatService(&mockConvRepo{}, &mockMsgRepo{}, map[string]LLMClient{"openai": &mockLLMClient{}}, slog.Default())
	_, err := s.StreamChat(context.Background(), "openai", "gpt-4", "", "")
	var inputErr *InputError
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected InputError, got %T: %v", err, err)
	}
	if inputErr.Field != "message" {
		t.Errorf("expected field 'message', got %q", inputErr.Field)
	}
}

func TestChatService_StreamChat_CancelledConversation(t *testing.T) {
	convRepo := &mockConvRepo{
		createFn: func(_ context.Context, title, provider, model string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: "conv-1"}, nil
		},
		getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: id, Status: "cancelled"}, nil
		},
	}
	msgRepo := &mockMsgRepo{
		getByConversationFn: func(_ context.Context, conversationID string) ([]pkg.Message, error) {
			return []pkg.Message{}, nil
		},
		nextSeqFn: func(_ context.Context, conversationID string) (int, error) {
			return 1, nil
		},
		insertFn: func(_ context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error) {
			return &pkg.Message{ID: "msg-1"}, nil
		},
	}
	s := NewChatService(convRepo, msgRepo, map[string]LLMClient{"openai": &mockLLMClient{}}, slog.Default())
	_, err := s.StreamChat(context.Background(), "openai", "gpt-4", "hello", "")
	if !errors.Is(err, ErrConversationCancelled) {
		t.Fatalf("expected ErrConversationCancelled, got %v", err)
	}
}

func TestChatService_StreamChat_NewConversation(t *testing.T) {
	created := false
	convRepo := &mockConvRepo{
		createFn: func(_ context.Context, title, provider, model string) (*pkg.Conversation, error) {
			created = true
			return &pkg.Conversation{ID: "new-conv"}, nil
		},
		getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: id, Status: "active"}, nil
		},
		touchFn: func(_ context.Context, id string) error {
			return nil
		},
	}
	msgRepo := &mockMsgRepo{
		getByConversationFn: func(_ context.Context, conversationID string) ([]pkg.Message, error) {
			return []pkg.Message{}, nil
		},
		nextSeqFn: func(_ context.Context, conversationID string) (int, error) {
			return 1, nil
		},
		insertFn: func(_ context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error) {
			return &pkg.Message{ID: "msg-1"}, nil
		},
	}
	llmClient := &mockLLMClient{
		streamChatFn: func(ctx context.Context, req providers.ChatRequest, conversationID, sessionID string) *llm.StreamResult {
			ch := make(chan llm.StreamEvent, 1)
			ch <- llm.StreamEvent{Done: true}
			close(ch)
			return &llm.StreamResult{Events: ch, RequestID: "req-1"}
		},
	}
	s := NewChatService(convRepo, msgRepo, map[string]LLMClient{"openai": llmClient}, slog.Default())
	result, err := s.StreamChat(context.Background(), "openai", "gpt-4", "hello", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("repo.Create was not called for new conversation")
	}
	if result.ConversationID != "new-conv" {
		t.Errorf("expected conversation ID 'new-conv', got %q", result.ConversationID)
	}
}

func TestChatService_StreamChat_ExistingConversation(t *testing.T) {
	convRepo := &mockConvRepo{
		getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: id, Status: "active"}, nil
		},
		touchFn: func(_ context.Context, id string) error {
			return nil
		},
	}
	msgRepo := &mockMsgRepo{
		getByConversationFn: func(_ context.Context, conversationID string) ([]pkg.Message, error) {
			return []pkg.Message{
				{Role: "user", Content: "hi", Seq: 1},
				{Role: "assistant", Content: "hello", Seq: 2},
			}, nil
		},
		nextSeqFn: func(_ context.Context, conversationID string) (int, error) {
			return 3, nil
		},
		insertFn: func(_ context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error) {
			return &pkg.Message{ID: "msg-3"}, nil
		},
	}
	llmClient := &mockLLMClient{
		streamChatFn: func(ctx context.Context, req providers.ChatRequest, conversationID, sessionID string) *llm.StreamResult {
			if len(req.Messages) != 3 {
				t.Errorf("expected 3 messages (2 history + 1 new), got %d", len(req.Messages))
			}
			ch := make(chan llm.StreamEvent, 1)
			ch <- llm.StreamEvent{Done: true}
			close(ch)
			return &llm.StreamResult{Events: ch, RequestID: "req-1"}
		},
	}
	s := NewChatService(convRepo, msgRepo, map[string]LLMClient{"openai": llmClient}, slog.Default())
	result, err := s.StreamChat(context.Background(), "openai", "gpt-4", "new message", "existing-conv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConversationID != "existing-conv" {
		t.Errorf("expected conversation ID 'existing-conv', got %q", result.ConversationID)
	}
}

func TestChatService_StreamChat_RepoNotFound(t *testing.T) {
	convRepo := &mockConvRepo{
		getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return nil, repo.ErrNotFound
		},
	}
	s := NewChatService(convRepo, &mockMsgRepo{}, map[string]LLMClient{"openai": &mockLLMClient{}}, slog.Default())
	_, err := s.StreamChat(context.Background(), "openai", "gpt-4", "hello", "nonexistent")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestToProviderMessages(t *testing.T) {
	msgs := []pkg.Message{
		{Role: "user", Content: "hello", Seq: 1},
		{Role: "assistant", Content: "world", Seq: 2},
	}
	got := toProviderMessages(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Role != "user" || got[0].Content != "hello" {
		t.Errorf("first message wrong: %+v", got[0])
	}
	if got[1].Role != "assistant" || got[1].Content != "world" {
		t.Errorf("second message wrong: %+v", got[1])
	}
}
