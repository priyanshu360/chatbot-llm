package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
)

type ErrorBody struct {
	Error  string `json:"error"`
	Field  string `json:"field,omitempty"`
	Status int    `json:"-"`
}

func writeError(w http.ResponseWriter, err error) {
	body := errorBody(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(body.Status)
	json.NewEncoder(w).Encode(body)
}

func errorBody(err error) ErrorBody {
	if err == nil {
		return ErrorBody{Error: "unknown error", Status: http.StatusInternalServerError}
	}

	var inputErr *service.InputError
	if errors.As(err, &inputErr) {
		return ErrorBody{
			Error:  inputErr.Message,
			Field:  inputErr.Field,
			Status: http.StatusBadRequest,
		}
	}

	var streamErr *service.StreamError
	if errors.As(err, &streamErr) {
		return ErrorBody{
			Error:  streamErr.Error(),
			Status: http.StatusInternalServerError,
		}
	}

	if errors.Is(err, service.ErrConversationNotFound) {
		return ErrorBody{
			Error:  "conversation not found",
			Status: http.StatusNotFound,
		}
	}

	if errors.Is(err, service.ErrConversationCancelled) {
		return ErrorBody{
			Error:  "conversation is cancelled",
			Status: http.StatusBadRequest,
		}
	}

	return ErrorBody{
		Error:  "internal server error",
		Status: http.StatusInternalServerError,
	}
}
