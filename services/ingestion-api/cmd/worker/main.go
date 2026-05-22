package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/repo"
	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/validation"
)

func main() {
	godotenv.Load()

	logLevel := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	redisAddr := fmt.Sprintf("%s:%s", envOrDefault("REDIS_HOST", "localhost"), envOrDefault("REDIS_PORT", "6379"))
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		envOrDefault("POSTGRES_USER", "llmuser"),
		envOrDefault("POSTGRES_PASSWORD", "llmpass"),
		envOrDefault("POSTGRES_HOST", "localhost"),
		envOrDefault("POSTGRES_PORT", "5432"),
		envOrDefault("POSTGRES_DB", "llmlog"),
	)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error("db config parse failed", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Error("db connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logRepo := repo.NewInferenceLogRepo(pool, logger)
	dlqRepo := repo.NewDLQRepo(pool, logger)
	consumer := repo.NewQueueConsumer(rdb, hostname)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Info("worker shutting down...")
		cancel()
	}()

	logger.Info("worker started")

	const batchSize = 500
	const flushInterval = 2 * time.Second

	if err := consumer.EnsureGroup(ctx); err != nil {
		logger.Error("ensure consumer group", "error", err)
	}

	var batch []redis.XMessage
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				flush(ctx, logRepo, dlqRepo, consumer, batch, logger)
				return
			case <-ticker.C:
				if len(batch) > 0 {
					logger.Debug("ticker flush", "batch_size", len(batch))
					flush(ctx, logRepo, dlqRepo, consumer, batch, logger)
					batch = batch[:0]
				}
			default:
				messages, err := consumer.ReadBatch(ctx, int64(batchSize))
				if err != nil {
					logger.Error("read batch error", "error", err)
					time.Sleep(time.Second)
					continue
				}
				if len(messages) == 0 {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				logger.Debug("read batch", "count", len(messages))
				batch = append(batch, messages...)

				if len(batch) >= batchSize {
					logger.Debug("batch full, flushing", "batch_size", len(batch))
					flush(ctx, logRepo, dlqRepo, consumer, batch, logger)
					batch = batch[:0]
				}
		}
	}
}

func flush(ctx context.Context, logRepo *repo.InferenceLogRepo, dlqRepo *repo.DLQRepo, consumer *repo.QueueConsumer, messages []redis.XMessage, logger *slog.Logger) {
	if len(messages) == 0 {
		return
	}

	var rows []repo.InferenceLogRow
	var ackIDs []string

	for _, msg := range messages {
		payloadStr, ok := msg.Values["payload"].(string)
		if !ok {
			ackIDs = append(ackIDs, msg.ID)
			continue
		}

		result := validation.ValidateInferenceLog([]byte(payloadStr))
		if !result.Valid {
			if err := dlqRepo.Insert(ctx, json.RawMessage(payloadStr), result.Errors[0].Error()); err != nil {
				logger.Error("dlq insert", "error", err)
			}
			ackIDs = append(ackIDs, msg.ID)
			continue
		}

		p := result.Payload
		rows = append(rows, repo.InferenceLogRow{
			RequestID:      p.RequestID,
			ConversationID: p.ConversationID,
			MessageID:      p.MessageID,
			Provider:       p.Provider,
			Model:          p.Model,
			LatencyMs:      p.LatencyMs,
			InputTokens:    p.InputTokens,
			OutputTokens:   p.OutputTokens,
			Status:         p.Status,
			ErrorCode:      p.ErrorCode,
			InputPreview:   p.InputPreview,
			OutputPreview:  p.OutputPreview,
			CreatedAt:      p.Timestamp,
		})
		ackIDs = append(ackIDs, msg.ID)
	}

	if len(rows) > 0 {
		if err := logRepo.BatchInsert(ctx, rows); err != nil {
			logger.Error("batch insert error, will retry", "error", err)
			return
		}
	}

	if err := consumer.Ack(ctx, ackIDs...); err != nil {
		logger.Error("ack error", "error", err)
	}
	logger.Info("flush complete", "messages", len(messages), "inference_logs", len(rows))
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
