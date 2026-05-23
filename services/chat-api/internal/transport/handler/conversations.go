package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

type ConversationsHandler struct {
	svc      ConversationService
	messages http.Handler
	logger   *slog.Logger
}

func NewConversationsHandler(svc ConversationService, messages http.Handler, logger *slog.Logger) *ConversationsHandler {
	return &ConversationsHandler{svc: svc, messages: messages, logger: logger}
}

func (h *ConversationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations")
	id = strings.TrimPrefix(id, "/")

	if strings.Contains(id, "/messages") {
		h.messages.ServeHTTP(w, r)
		return
	}

	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.delete(w, r, id)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (h *ConversationsHandler) list(w http.ResponseWriter, r *http.Request) {
	conversations, err := h.svc.List(r.Context())
	if err != nil {
		h.logger.Error("list conversations", "error", err)
		http.Error(w, `{"error":"failed to list conversations"}`, http.StatusInternalServerError)
		return
	}
	if conversations == nil {
		conversations = []pkg.Conversation{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversations)
}

func (h *ConversationsHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	conversation, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversation)
}

func (h *ConversationsHandler) create(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req pkg.CreateConversationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	c, err := h.svc.Create(r.Context(), req.Title)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *ConversationsHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req pkg.UpdateConversationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	switch req.Status {
	case "cancelled":
		err = h.svc.Cancel(r.Context(), id)
	case "active":
		err = h.svc.Resume(r.Context(), id)
	default:
		http.Error(w, `{"error":"status must be 'active' or 'cancelled'"}`, http.StatusBadRequest)
		return
	}

	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *ConversationsHandler) delete(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
