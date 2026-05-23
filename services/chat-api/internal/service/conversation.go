package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/repo"
)

type ConversationService struct {
	convRepo ConversationRepository
	msgRepo  MessageRepository
	logger   *slog.Logger
}

func NewConversationService(convRepo ConversationRepository, msgRepo MessageRepository, logger *slog.Logger) *ConversationService {
	return &ConversationService{convRepo: convRepo, msgRepo: msgRepo, logger: logger}
}

func (s *ConversationService) Create(ctx context.Context, title string) (*pkg.Conversation, error) {
	return s.convRepo.Create(ctx, title)
}

func (s *ConversationService) Get(ctx context.Context, id string) (*pkg.Conversation, error) {
	c, err := s.convRepo.GetByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ConversationService) List(ctx context.Context) ([]pkg.Conversation, error) {
	return s.convRepo.List(ctx)
}

func (s *ConversationService) Cancel(ctx context.Context, id string) error {
	err := s.convRepo.UpdateStatus(ctx, id, "cancelled")
	if errors.Is(err, repo.ErrNotFound) {
		return ErrConversationNotFound
	}
	return err
}

func (s *ConversationService) Resume(ctx context.Context, id string) error {
	err := s.convRepo.UpdateStatus(ctx, id, "active")
	if errors.Is(err, repo.ErrNotFound) {
		return ErrConversationNotFound
	}
	return err
}

func (s *ConversationService) Delete(ctx context.Context, id string) error {
	err := s.convRepo.Delete(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return ErrConversationNotFound
	}
	return err
}

func (s *ConversationService) GetMessages(ctx context.Context, conversationID string) ([]pkg.Message, error) {
	return s.msgRepo.GetByConversation(ctx, conversationID)
}
