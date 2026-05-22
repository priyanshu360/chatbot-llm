package repo

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DLQRepo struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewDLQRepo(pool *pgxpool.Pool, logger *slog.Logger) *DLQRepo {
	return &DLQRepo{pool: pool, logger: logger}
}

func (r *DLQRepo) Insert(ctx context.Context, raw json.RawMessage, errMsg string) error {
	r.logger.Debug("inserting into DLQ", "error", errMsg)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ingestion_dlq (raw, error_msg) VALUES ($1, $2)`,
		raw, errMsg,
	)
	return err
}
