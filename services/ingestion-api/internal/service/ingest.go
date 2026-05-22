package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/validation"
)

type IngestService struct {
	dlqRepo  DLQRepository
	producer QueuePublisher
	logger   *slog.Logger
}

func NewIngestService(dlqRepo DLQRepository, producer QueuePublisher, logger *slog.Logger) *IngestService {
	return &IngestService{dlqRepo: dlqRepo, producer: producer, logger: logger}
}

type IngestResult struct {
	Accepted int
	Errors   []validation.ValidationError
}

func (s *IngestService) ProcessLog(ctx context.Context, raw json.RawMessage) (IngestResult, error) {
	result := validation.ValidateInferenceLog(raw)
	if !result.Valid {
		s.logger.Debug("inference log validation failed", "errors", result.Errors)
		if err := s.dlqRepo.Insert(ctx, raw, result.Errors[0].Error()); err != nil {
			return IngestResult{}, err
		}
		return IngestResult{Errors: result.Errors}, nil
	}

	s.logger.Debug("publishing inference log to Redis", "request_id", result.Payload.RequestID, "provider", result.Payload.Provider)
	if err := s.producer.Publish(ctx, raw); err != nil {
		return IngestResult{}, ErrQueueUnavailable
	}

	return IngestResult{Accepted: 1}, nil
}
