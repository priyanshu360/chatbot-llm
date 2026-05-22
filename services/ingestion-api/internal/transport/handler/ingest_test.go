package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/service"
	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/validation"
)

type mockIngestSvc struct {
	processLogFn func(ctx context.Context, raw json.RawMessage) (service.IngestResult, error)
}

func (m *mockIngestSvc) ProcessLog(ctx context.Context, raw json.RawMessage) (service.IngestResult, error) {
	return m.processLogFn(ctx, raw)
}

var _ IngestService = (*mockIngestSvc)(nil)

func TestIngestHandler_Post_Accepted(t *testing.T) {
	svc := &mockIngestSvc{
		processLogFn: func(_ context.Context, raw json.RawMessage) (service.IngestResult, error) {
			return service.IngestResult{Accepted: 1}, nil
		},
	}
	h := NewIngestHandler(svc)

	body := `{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"session_id": "s1",
		"conversation_id": "c1",
		"model": "gpt-4",
		"provider": "openai",
		"latency_ms": 100,
		"input_tokens": 10,
		"output_tokens": 20,
		"status": "success"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/inference", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["accepted"].(float64) != 1 {
		t.Errorf("expected accepted=1, got %v", resp["accepted"])
	}
}

func TestIngestHandler_Post_ValidationError(t *testing.T) {
	svc := &mockIngestSvc{
		processLogFn: func(_ context.Context, raw json.RawMessage) (service.IngestResult, error) {
			return service.IngestResult{
				Errors: []validation.ValidationError{
					{Field: "request_id", Message: "request_id is required"},
				},
			}, nil
		},
	}
	h := NewIngestHandler(svc)

	body := `{"request_id": ""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/inference", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["accepted"].(float64) != 0 {
		t.Errorf("expected accepted=0, got %v", resp["accepted"])
	}
}

func TestIngestHandler_Post_QueueUnavailable(t *testing.T) {
	svc := &mockIngestSvc{
		processLogFn: func(_ context.Context, raw json.RawMessage) (service.IngestResult, error) {
			return service.IngestResult{}, service.ErrQueueUnavailable
		},
	}
	h := NewIngestHandler(svc)

	body := `{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"session_id": "s1",
		"conversation_id": "c1",
		"model": "gpt-4",
		"provider": "openai",
		"latency_ms": 100,
		"input_tokens": 10,
		"output_tokens": 20,
		"status": "success"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/inference", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestIngestHandler_Post_MethodNotAllowed(t *testing.T) {
	h := NewIngestHandler(&mockIngestSvc{})
	req := httptest.NewRequest(http.MethodGet, "/v1/ingest/inference", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	HealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
