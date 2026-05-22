package handler

import (
	"context"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
)

type ConversationService interface {
	Create(ctx context.Context, title, provider, model string) (*pkg.Conversation, error)
	Get(ctx context.Context, id string) (*pkg.Conversation, error)
	List(ctx context.Context) ([]pkg.Conversation, error)
	Cancel(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	GetMessages(ctx context.Context, conversationID string) ([]pkg.Message, error)
}

type ChatService interface {
	StreamChat(ctx context.Context, providerName, model, message, conversationID string) (*service.StreamResult, error)
}
