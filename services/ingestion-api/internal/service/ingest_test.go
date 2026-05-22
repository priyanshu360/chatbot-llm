package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
)

type mockDLQRepo struct {
	insertFn func(ctx context.Context, raw json.RawMessage, errMsg string) error
}

func (m *mockDLQRepo) Insert(ctx context.Context, raw json.RawMessage, errMsg string) error {
	return m.insertFn(ctx, raw, errMsg)
}

var _ DLQRepository = (*mockDLQRepo)(nil)

type mockQueuePublisher struct {
	publishFn func(ctx context.Context, payload json.RawMessage) error
}

func (m *mockQueuePublisher) Publish(ctx context.Context, payload json.RawMessage) error {
	return m.publishFn(ctx, payload)
}

var _ QueuePublisher = (*mockQueuePublisher)(nil)

func TestIngestService_ProcessLog_Valid(t *testing.T) {
	published := false
	producer := &mockQueuePublisher{
		publishFn: func(_ context.Context, payload json.RawMessage) error {
			published = true
			return nil
		},
	}
	svc := NewIngestService(&mockDLQRepo{}, producer, slog.Default())

	raw := json.RawMessage(`{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"session_id": "s1",
		"conversation_id": "c1",
		"model": "gpt-4",
		"provider": "openai",
		"latency_ms": 100,
		"input_tokens": 10,
		"output_tokens": 20,
		"status": "success"
	}`)
	result, err := svc.ProcessLog(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Accepted != 1 {
		t.Errorf("expected Accepted=1, got %d", result.Accepted)
	}
	if !published {
		t.Error("expected Publish to be called")
	}
}

func TestIngestService_ProcessLog_InvalidPayload(t *testing.T) {
	dlqWritten := false
	dlq := &mockDLQRepo{
		insertFn: func(_ context.Context, raw json.RawMessage, errMsg string) error {
			dlqWritten = true
			return nil
		},
	}
	svc := NewIngestService(dlq, &mockQueuePublisher{}, slog.Default())

	raw := json.RawMessage(`{"request_id": ""}`)
	result, err := svc.ProcessLog(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Accepted != 0 {
		t.Errorf("expected Accepted=0, got %d", result.Accepted)
	}
	if !dlqWritten {
		t.Error("expected DLQ Insert to be called")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestIngestService_ProcessLog_QueueUnavailable(t *testing.T) {
	producer := &mockQueuePublisher{
		publishFn: func(_ context.Context, payload json.RawMessage) error {
			return errors.New("redis down")
		},
	}
	svc := NewIngestService(&mockDLQRepo{}, producer, slog.Default())

	raw := json.RawMessage(`{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"session_id": "s1",
		"conversation_id": "c1",
		"model": "gpt-4",
		"provider": "openai",
		"latency_ms": 100,
		"input_tokens": 10,
		"output_tokens": 20,
		"status": "success"
	}`)
	_, err := svc.ProcessLog(context.Background(), raw)
	if !errors.Is(err, ErrQueueUnavailable) {
		t.Fatalf("expected ErrQueueUnavailable, got %v", err)
	}
}

func TestIngestService_ProcessLog_DLQFailure(t *testing.T) {
	dlq := &mockDLQRepo{
		insertFn: func(_ context.Context, raw json.RawMessage, errMsg string) error {
			return errors.New("db error")
		},
	}
	svc := NewIngestService(dlq, &mockQueuePublisher{}, slog.Default())

	raw := json.RawMessage(`{"request_id": ""}`)
	_, err := svc.ProcessLog(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error from DLQ failure")
	}
	if err.Error() != "db error" {
		t.Errorf("expected 'db error', got %v", err)
	}
}
