package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ProvidersHandler struct {
	chatSvc ChatService
	logger  *slog.Logger
}

func NewProvidersHandler(chatSvc ChatService, logger *slog.Logger) *ProvidersHandler {
	return &ProvidersHandler{chatSvc: chatSvc, logger: logger}
}

func (h *ProvidersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	providers := h.chatSvc.ListProviders()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}
