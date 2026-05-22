package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/util"
)

type ChatHandler struct {
	chatSvc ChatService
	logger  *slog.Logger
}

func NewChatHandler(chatSvc ChatService, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{chatSvc: chatSvc, logger: logger}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<18))
	if err != nil {
		http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req pkg.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
		return
	}

	h.logger.Debug("chat request", "provider", req.Provider, "model", req.Model, "conversation_id", req.ConversationID)

	result, err := h.chatSvc.StreamChat(r.Context(), req.Provider, req.Model, req.Message, req.ConversationID)
	if err != nil {
		h.logger.Debug("chat stream init failed", "error", err)
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: meta\ndata: %s\n\n", util.ToJSON(pkg.ChatResponse{
		ConversationID: result.ConversationID,
		MessageID:      result.UserMessageID,
	}))
	flusher.Flush()

	fullContent := ""
	for evt := range result.Events {
		if evt.Done {
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", util.ToJSON(map[string]interface{}{
				"content": fullContent,
				"usage":   evt.Usage,
			}))
			flusher.Flush()
			return
		}
		if evt.Error != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", util.ToJSON(map[string]string{"error": evt.Error.Error()}))
			flusher.Flush()
			return
		}
		if evt.Delta != "" {
			fullContent += evt.Delta
			fmt.Fprintf(w, "event: token\ndata: %s\n\n", util.ToJSON(map[string]string{"delta": evt.Delta}))
			flusher.Flush()
		}
	}
}
