package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/repo"
)

type mockConvRepo struct {
	createFn       func(ctx context.Context, title, provider, model string) (*pkg.Conversation, error)
	getByIDFn      func(ctx context.Context, id string) (*pkg.Conversation, error)
	listFn         func(ctx context.Context) ([]pkg.Conversation, error)
	updateStatusFn func(ctx context.Context, id, status string) error
	deleteFn       func(ctx context.Context, id string) error
	touchFn        func(ctx context.Context, id string) error
}

func (m *mockConvRepo) Create(ctx context.Context, title, provider, model string) (*pkg.Conversation, error) {
	return m.createFn(ctx, title, provider, model)
}
func (m *mockConvRepo) GetByID(ctx context.Context, id string) (*pkg.Conversation, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockConvRepo) List(ctx context.Context) ([]pkg.Conversation, error) {
	return m.listFn(ctx)
}
func (m *mockConvRepo) UpdateStatus(ctx context.Context, id, status string) error {
	return m.updateStatusFn(ctx, id, status)
}
func (m *mockConvRepo) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}
func (m *mockConvRepo) Touch(ctx context.Context, id string) error {
	return m.touchFn(ctx, id)
}

type mockMsgRepo struct {
	insertFn            func(ctx context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error)
	getByConversationFn func(ctx context.Context, conversationID string) ([]pkg.Message, error)
	nextSeqFn           func(ctx context.Context, conversationID string) (int, error)
}

func (m *mockMsgRepo) Insert(ctx context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error) {
	return m.insertFn(ctx, conversationID, role, content, contentRedacted, seq)
}
func (m *mockMsgRepo) GetByConversation(ctx context.Context, conversationID string) ([]pkg.Message, error) {
	return m.getByConversationFn(ctx, conversationID)
}
func (m *mockMsgRepo) NextSeq(ctx context.Context, conversationID string) (int, error) {
	return m.nextSeqFn(ctx, conversationID)
}

var _ ConversationRepository = (*mockConvRepo)(nil)
var _ MessageRepository = (*mockMsgRepo)(nil)

func TestConversationService_Create(t *testing.T) {
	t.Run("empty provider returns InputError", func(t *testing.T) {
		s := NewConversationService(&mockConvRepo{}, &mockMsgRepo{}, slog.Default())
		_, err := s.Create(context.Background(), "", "", "gpt-4")
		var inputErr *InputError
		if !errors.As(err, &inputErr) {
			t.Fatalf("expected InputError, got %T: %v", err, err)
		}
		if inputErr.Field != "provider" {
			t.Errorf("expected field 'provider', got %q", inputErr.Field)
		}
	})

	t.Run("empty model returns InputError", func(t *testing.T) {
		s := NewConversationService(&mockConvRepo{}, &mockMsgRepo{}, slog.Default())
		_, err := s.Create(context.Background(), "", "openai", "")
		var inputErr *InputError
		if !errors.As(err, &inputErr) {
			t.Fatalf("expected InputError, got %T: %v", err, err)
		}
		if inputErr.Field != "model" {
			t.Errorf("expected field 'model', got %q", inputErr.Field)
		}
	})

	t.Run("valid input calls repo", func(t *testing.T) {
		called := false
		repo := &mockConvRepo{
			createFn: func(_ context.Context, title, provider, model string) (*pkg.Conversation, error) {
				called = true
				if provider != "openai" {
					t.Errorf("expected provider openai, got %q", provider)
				}
				if model != "gpt-4" {
					t.Errorf("expected model gpt-4, got %q", model)
				}
				return &pkg.Conversation{ID: "abc", Provider: provider, Model: model}, nil
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		c, err := s.Create(context.Background(), "test", "openai", "gpt-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatal("repo.Create was not called")
		}
		if c.ID != "abc" {
			t.Errorf("expected ID abc, got %q", c.ID)
		}
	})
}

func TestConversationService_Get(t *testing.T) {
	t.Run("found returns conversation", func(t *testing.T) {
		repo := &mockConvRepo{
			getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
				return &pkg.Conversation{ID: id, Title: "test"}, nil
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		c, err := s.Get(context.Background(), "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.ID != "abc" {
			t.Errorf("expected ID abc, got %q", c.ID)
		}
	})

	t.Run("not found returns ErrConversationNotFound", func(t *testing.T) {
		repo := &mockConvRepo{
			getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
				return nil, repo.ErrNotFound
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		_, err := s.Get(context.Background(), "nonexistent")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected ErrConversationNotFound, got %v", err)
		}
	})

	t.Run("unexpected error propagated", func(t *testing.T) {
		expected := errors.New("db timeout")
		repo := &mockConvRepo{
			getByIDFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
				return nil, expected
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		_, err := s.Get(context.Background(), "abc")
		if !errors.Is(err, expected) {
			t.Fatalf("expected %v, got %v", expected, err)
		}
	})
}

func TestConversationService_List(t *testing.T) {
	expected := []pkg.Conversation{{ID: "1"}, {ID: "2"}}
	repo := &mockConvRepo{
		listFn: func(_ context.Context) ([]pkg.Conversation, error) {
			return expected, nil
		},
	}
	s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestConversationService_Cancel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mockConvRepo{
			updateStatusFn: func(_ context.Context, id, status string) error {
				if status != "cancelled" {
					t.Errorf("expected status cancelled, got %q", status)
				}
				return nil
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Cancel(context.Background(), "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found returns ErrConversationNotFound", func(t *testing.T) {
		repo := &mockConvRepo{
			updateStatusFn: func(_ context.Context, id, status string) error {
				return repo.ErrNotFound
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Cancel(context.Background(), "nonexistent")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected ErrConversationNotFound, got %v", err)
		}
	})
}

func TestConversationService_Resume(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mockConvRepo{
			updateStatusFn: func(_ context.Context, id, status string) error {
				if status != "active" {
					t.Errorf("expected status active, got %q", status)
				}
				return nil
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Resume(context.Background(), "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found returns ErrConversationNotFound", func(t *testing.T) {
		repo := &mockConvRepo{
			updateStatusFn: func(_ context.Context, id, status string) error {
				return repo.ErrNotFound
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Resume(context.Background(), "nonexistent")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected ErrConversationNotFound, got %v", err)
		}
	})
}

func TestConversationService_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mockConvRepo{
			deleteFn: func(_ context.Context, id string) error {
				return nil
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Delete(context.Background(), "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found returns ErrConversationNotFound", func(t *testing.T) {
		repo := &mockConvRepo{
			deleteFn: func(_ context.Context, id string) error {
				return repo.ErrNotFound
			},
		}
		s := NewConversationService(repo, &mockMsgRepo{}, slog.Default())
		err := s.Delete(context.Background(), "nonexistent")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected ErrConversationNotFound, got %v", err)
		}
	})
}

func TestConversationService_GetMessages(t *testing.T) {
	expected := []pkg.Message{{ID: "m1", Content: "hello"}}
	msgRepo := &mockMsgRepo{
		getByConversationFn: func(_ context.Context, conversationID string) ([]pkg.Message, error) {
			return expected, nil
		},
	}
	s := NewConversationService(&mockConvRepo{}, msgRepo, slog.Default())
	messages, err := s.GetMessages(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}
