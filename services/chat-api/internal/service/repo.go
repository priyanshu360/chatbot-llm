package service

import (
	"context"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

type ConversationRepository interface {
	Create(ctx context.Context, title string) (*pkg.Conversation, error)
	GetByID(ctx context.Context, id string) (*pkg.Conversation, error)
	List(ctx context.Context) ([]pkg.Conversation, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
	Touch(ctx context.Context, id string) error
}

type MessageRepository interface {
	Insert(ctx context.Context, conversationID, role, content, contentRedacted, provider, model string, seq int) (*pkg.Message, error)
	GetByConversation(ctx context.Context, conversationID string) ([]pkg.Message, error)
	NextSeq(ctx context.Context, conversationID string) (int, error)
}
