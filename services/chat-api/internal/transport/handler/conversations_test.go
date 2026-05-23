package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/priyanshu360/chatbot-llm/pkg"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
)

type mockConvSvc struct {
	createFn  func(ctx context.Context, title string) (*pkg.Conversation, error)
	getFn     func(ctx context.Context, id string) (*pkg.Conversation, error)
	listFn    func(ctx context.Context) ([]pkg.Conversation, error)
	cancelFn  func(ctx context.Context, id string) error
	resumeFn  func(ctx context.Context, id string) error
	deleteFn  func(ctx context.Context, id string) error
	msgsFn    func(ctx context.Context, conversationID string) ([]pkg.Message, error)
}

func (m *mockConvSvc) Create(ctx context.Context, title string) (*pkg.Conversation, error) {
	return m.createFn(ctx, title)
}
func (m *mockConvSvc) Get(ctx context.Context, id string) (*pkg.Conversation, error) {
	return m.getFn(ctx, id)
}
func (m *mockConvSvc) List(ctx context.Context) ([]pkg.Conversation, error) {
	return m.listFn(ctx)
}
func (m *mockConvSvc) Cancel(ctx context.Context, id string) error {
	return m.cancelFn(ctx, id)
}
func (m *mockConvSvc) Resume(ctx context.Context, id string) error {
	return m.resumeFn(ctx, id)
}
func (m *mockConvSvc) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}
func (m *mockConvSvc) GetMessages(ctx context.Context, conversationID string) ([]pkg.Message, error) {
	return m.msgsFn(ctx, conversationID)
}

var _ ConversationService = (*mockConvSvc)(nil)

func TestConversationsHandler_GetList(t *testing.T) {
	svc := &mockConvSvc{
		listFn: func(_ context.Context) ([]pkg.Conversation, error) {
			return []pkg.Conversation{{ID: "1", Title: "test"}}, nil
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/conversations", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var convs []pkg.Conversation
	if err := json.NewDecoder(w.Body).Decode(&convs); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
}

func TestConversationsHandler_GetByID(t *testing.T) {
	svc := &mockConvSvc{
		getFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: id, Title: "found"}, nil
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestConversationsHandler_GetByID_NotFound(t *testing.T) {
	svc := &mockConvSvc{
		getFn: func(_ context.Context, id string) (*pkg.Conversation, error) {
			return nil, service.ErrConversationNotFound
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestConversationsHandler_Create(t *testing.T) {
	svc := &mockConvSvc{
		createFn: func(_ context.Context, title string) (*pkg.Conversation, error) {
			return &pkg.Conversation{ID: "new-1", Title: title}, nil
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	body := `{"title":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var conv pkg.Conversation
	json.NewDecoder(w.Body).Decode(&conv)
	if conv.ID != "new-1" {
		t.Errorf("expected ID 'new-1', got %q", conv.ID)
	}
}

func TestConversationsHandler_Create_InvalidJSON(t *testing.T) {
	svc := &mockConvSvc{}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/conversations", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConversationsHandler_Cancel(t *testing.T) {
	svc := &mockConvSvc{
		cancelFn: func(_ context.Context, id string) error {
			return nil
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	body := `{"status":"cancelled"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/conversations/abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestConversationsHandler_Cancel_NotFound(t *testing.T) {
	svc := &mockConvSvc{
		cancelFn: func(_ context.Context, id string) error {
			return service.ErrConversationNotFound
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	body := `{"status":"cancelled"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/conversations/abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestConversationsHandler_Delete(t *testing.T) {
	svc := &mockConvSvc{
		deleteFn: func(_ context.Context, id string) error {
			return nil
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodDelete, "/api/conversations/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestConversationsHandler_Delete_NotFound(t *testing.T) {
	svc := &mockConvSvc{
		deleteFn: func(_ context.Context, id string) error {
			return service.ErrConversationNotFound
		},
	}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodDelete, "/api/conversations/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestConversationsHandler_MethodNotAllowed(t *testing.T) {
	svc := &mockConvSvc{}
	h := NewConversationsHandler(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), slog.Default())

	req := httptest.NewRequest(http.MethodPut, "/api/conversations/abc", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
