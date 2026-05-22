package repo

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InferenceLogRepo struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

type InferenceLogRow struct {
	RequestID      string
	ConversationID string
	MessageID      string
	Provider       string
	Model          string
	LatencyMs      int
	InputTokens    int
	OutputTokens   int
	Status         string
	ErrorCode      string
	InputPreview   string
	OutputPreview  string
	CreatedAt      string
}

func NewInferenceLogRepo(pool *pgxpool.Pool, logger *slog.Logger) *InferenceLogRepo {
	return &InferenceLogRepo{pool: pool, logger: logger}
}

func (r *InferenceLogRepo) BatchInsert(ctx context.Context, rows []InferenceLogRow) error {
	if len(rows) == 0 {
		return nil
	}

	r.logger.Debug("batch insert inference logs", "count", len(rows))

	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(
			`INSERT INTO inference_logs
			 (request_id, conversation_id, message_id, provider, model,
			  latency_ms, input_tokens, output_tokens, status, error_code,
			  input_preview, output_preview, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			 ON CONFLICT (request_id) DO NOTHING`,
			row.RequestID, row.ConversationID, nilIfEmpty(row.MessageID), row.Provider, row.Model,
			row.LatencyMs, row.InputTokens, row.OutputTokens, row.Status, row.ErrorCode,
			row.InputPreview, row.OutputPreview, row.CreatedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(rows); i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
