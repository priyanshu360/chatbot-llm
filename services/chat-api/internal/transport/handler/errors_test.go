package handler

import (
	"net/http"
	"testing"

	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
)

func TestErrorBody(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		body := errorBody(nil)
		if body.Status != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", body.Status)
		}
		if body.Error != "unknown error" {
			t.Errorf("expected 'unknown error', got %q", body.Error)
		}
	})

	t.Run("ErrConversationNotFound", func(t *testing.T) {
		body := errorBody(service.ErrConversationNotFound)
		if body.Status != http.StatusNotFound {
			t.Errorf("expected 404, got %d", body.Status)
		}
		if body.Error != "conversation not found" {
			t.Errorf("expected 'conversation not found', got %q", body.Error)
		}
	})

	t.Run("ErrConversationCancelled", func(t *testing.T) {
		body := errorBody(service.ErrConversationCancelled)
		if body.Status != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", body.Status)
		}
		if body.Error != "conversation is cancelled" {
			t.Errorf("expected 'conversation is cancelled', got %q", body.Error)
		}
	})

	t.Run("InputError", func(t *testing.T) {
		body := errorBody(&service.InputError{Field: "provider", Message: "provider is required"})
		if body.Status != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", body.Status)
		}
		if body.Error != "provider is required" {
			t.Errorf("expected 'provider is required', got %q", body.Error)
		}
		if body.Field != "provider" {
			t.Errorf("expected field 'provider', got %q", body.Field)
		}
	})

	t.Run("StreamError", func(t *testing.T) {
		body := errorBody(&service.StreamError{Provider: "openai", Model: "gpt-4", Cause: service.ErrConversationNotFound})
		if body.Status != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", body.Status)
		}
	})

	t.Run("wrapped ErrConversationNotFound", func(t *testing.T) {
		err := service.ErrConversationNotFound
		body := errorBody(err)
		if body.Status != http.StatusNotFound {
			t.Errorf("expected 404, got %d", body.Status)
		}
	})

	t.Run("generic error", func(t *testing.T) {
		body := errorBody(http.ErrBodyNotAllowed)
		if body.Status != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", body.Status)
		}
	})
}
