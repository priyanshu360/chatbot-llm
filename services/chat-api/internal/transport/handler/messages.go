package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type MessagesHandler struct {
	svc    ConversationService
	logger *slog.Logger
}

func NewMessagesHandler(svc ConversationService, logger *slog.Logger) *MessagesHandler {
	return &MessagesHandler{svc: svc, logger: logger}
}

func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	conversationID := strings.Split(path, "/")[0]

	messages, err := h.svc.GetMessages(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("get messages", "error", err)
		http.Error(w, `{"error":"failed to get messages"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
