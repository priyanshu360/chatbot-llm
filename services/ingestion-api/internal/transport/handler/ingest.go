package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/service"
)

type IngestHandler struct {
	svc IngestService
}

func NewIngestHandler(svc IngestService) *IngestHandler {
	return &IngestHandler{svc: svc}
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	result, err := h.svc.ProcessLog(r.Context(), body)
	if err != nil {
		if errors.Is(err, service.ErrQueueUnavailable) {
			http.Error(w, `{"error":"queue unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(result.Errors) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accepted": result.Accepted,
			"errors":   result.Errors,
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accepted": result.Accepted,
	})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
