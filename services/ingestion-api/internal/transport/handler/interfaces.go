package handler

import (
	"context"
	"encoding/json"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/service"
)

type IngestService interface {
	ProcessLog(ctx context.Context, raw json.RawMessage) (service.IngestResult, error)
}
