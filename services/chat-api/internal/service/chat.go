package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm/providers"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/util"
)

type LLMClient interface {
	StreamChat(ctx context.Context, req providers.ChatRequest, conversationID, sessionID, messageID string) *llm.StreamResult
}

type ChatService struct {
	convRepo       ConversationRepository
	msgRepo        MessageRepository
	clients        map[string]LLMClient
	providerModels map[string][]string
	logger         *slog.Logger
}

func NewChatService(convRepo ConversationRepository, msgRepo MessageRepository, clients map[string]LLMClient, providerModels map[string][]string, logger *slog.Logger) *ChatService {
	return &ChatService{convRepo: convRepo, msgRepo: msgRepo, clients: clients, providerModels: providerModels, logger: logger}
}

type ProviderInfo struct {
	Models []string `json:"models"`
}

func (s *ChatService) ListProviders() map[string]ProviderInfo {
	result := make(map[string]ProviderInfo, len(s.providerModels))
	for name, models := range s.providerModels {
		result[name] = ProviderInfo{Models: models}
	}
	return result
}

type StreamResult struct {
	Events          <-chan llm.StreamEvent
	ConversationID  string
	UserMessageID   string
}

func (s *ChatService) StreamChat(ctx context.Context, providerName, model, message, conversationID string) (*StreamResult, error) {
	client, ok := s.clients[providerName]
	if !ok {
		return nil, &InputError{Field: "provider", Message: fmt.Sprintf("unsupported provider: %s", providerName)}
	}

	if message == "" {
		return nil, &InputError{Field: "message", Message: "message is required"}
	}

	if conversationID == "" {
		c, err := s.convRepo.Create(ctx, util.TruncateTitle(message), providerName, model)
		if err != nil {
			return nil, fmt.Errorf("create conversation: %w", err)
		}
		conversationID = c.ID
		s.logger.Debug("created conversation", "conversation_id", conversationID)
	}

	conversation, err := s.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", ErrConversationNotFound)
	}
	if conversation.Status == "cancelled" {
		return nil, ErrConversationCancelled
	}

	messages, err := s.msgRepo.GetByConversation(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}
	s.logger.Debug("loaded history", "conversation_id", conversationID, "message_count", len(messages))

	seq, err := s.msgRepo.NextSeq(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get sequence: %w", err)
	}

	userMsg, err := s.msgRepo.Insert(ctx, conversationID, "user", message, llm.RedactPII(message), seq)
	if err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}
	s.logger.Debug("saved user message", "message_id", userMsg.ID, "seq", seq)

	if err := s.convRepo.Touch(ctx, conversationID); err != nil {
		s.logger.Warn("update timestamp failed", "error", err)
	}

	chatHistory := toProviderMessages(messages)
	chatHistory = append(chatHistory, providers.ChatMessage{Role: "user", Content: message})

	{
		const budgetRatio = 0.8
		budget := int(float64(contextWindow(providerName, model)) * budgetRatio)

		var keep int
		for i := len(chatHistory) - 1; i >= 0; i-- {
			t := estimateTokens(chatHistory[i].Content)
			if i == len(chatHistory)-1 || budget-t >= 0 {
				budget -= t
				keep = i
			} else {
				break
			}
		}
		chatHistory = chatHistory[keep:]
	}

	s.logger.Debug("calling LLM provider", "provider", providerName, "model", model, "history_len", len(chatHistory))
	result := client.StreamChat(ctx, providers.ChatRequest{
		Messages: chatHistory,
		Model:    model,
	}, conversationID, conversationID, userMsg.ID)

	wrapped := make(chan llm.StreamEvent)
	go func() {
		defer close(wrapped)
		var fullContent string
		for evt := range result.Events {
			if evt.Delta != "" {
				fullContent += evt.Delta
			}
			if evt.Done {
				if _, err := s.msgRepo.Insert(ctx, conversationID, "assistant", fullContent, llm.RedactPII(fullContent), seq+1); err != nil {
					s.logger.Error("save assistant message", "error", err)
				}
			}
			wrapped <- evt
		}
	}()

	return &StreamResult{
		Events:         wrapped,
		ConversationID: conversationID,
		UserMessageID:  userMsg.ID,
	}, nil
}

func toProviderMessages(msgs []pkg.Message) []providers.ChatMessage {
	result := make([]providers.ChatMessage, len(msgs))
	for i, m := range msgs {
		result[i] = providers.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return result
}
