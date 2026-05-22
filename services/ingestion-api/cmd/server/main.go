package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/repo"
	"github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/service"
	handler "github.com/priyanshu360/chatbot-llm/services/ingestion-api/internal/transport/handler"
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
	poolCfg.MaxConns = 15
	poolCfg.MinConns = 3
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Error("db connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	dlqRepo := repo.NewDLQRepo(pool, logger)
	producer := repo.NewQueueProducer(rdb)

	ingestSvc := service.NewIngestService(dlqRepo, producer, logger)

	mux := http.NewServeMux()
	mux.Handle("/v1/ingest/inference", handler.NewIngestHandler(ingestSvc))
	mux.HandleFunc("/v1/health", handler.HealthHandler)

	port := envOrDefault("INGESTION_API_PORT", "4001")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      withCORS(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("ingestion api listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
