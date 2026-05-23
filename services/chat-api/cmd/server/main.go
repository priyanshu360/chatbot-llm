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

	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/repo"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/service/llm/providers"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/transport/handler"
	"github.com/priyanshu360/chatbot-llm/services/chat-api/internal/transport/middleware"
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

	logger.Info("logger set up", "level", os.Getenv("LOG_LEVEL"))
	ctxbg := context.Background()

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
	poolCfg.MaxConns = 25
	poolCfg.MinConns = 5
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctxbg, poolCfg)
	if err != nil {
		logger.Error("db connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	convRepo := repo.NewConversationRepo(pool, logger)
	msgRepo := repo.NewMessageRepo(pool, logger)

	ingestionURL := fmt.Sprintf("http://%s:%s",
		envOrDefault("INGESTION_HOST", "localhost"),
		envOrDefault("INGESTION_API_PORT", "4001"),
	)
	ingestionLogger := llm.NewLogger(ingestionURL, logger)

	knownModels := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini"},
		"anthropic": {"claude-sonnet-4-6", "claude-haiku-4-5"},
		"gemini":    {"gemini-2.5-flash", "gemini-1.5-flash"},
		"ollama":    {"llama3.2", "llama3.1", "mistral", "phi4"},
		"deepseek":  {"deepseek-chat", "deepseek-reasoner"},
	}

	llmClients := make(map[string]service.LLMClient)

	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		llmClients["openai"] = llm.NewLLMClient(providers.NewOpenAI(key), ingestionLogger, logger)
		logger.Info("registered provider", "provider", "openai")
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		llmClients["anthropic"] = llm.NewLLMClient(providers.NewAnthropic(key), ingestionLogger, logger)
		logger.Info("registered provider", "provider", "anthropic")
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		llmClients["gemini"] = llm.NewLLMClient(providers.NewGemini(key), ingestionLogger, logger)
		logger.Info("registered provider", "provider", "gemini")
	}
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		llmClients["ollama"] = llm.NewLLMClient(providers.NewOllama(baseURL), ingestionLogger, logger)
		logger.Info("registered provider", "provider", "ollama")
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		llmClients["deepseek"] = llm.NewLLMClient(providers.NewDeepSeek(key), ingestionLogger, logger)
		logger.Info("registered provider", "provider", "deepseek")
	}

	if len(llmClients) == 0 {
		logger.Warn("no LLM providers configured")
	}

	providerModels := make(map[string][]string, len(llmClients))
	for name := range llmClients {
		if models, ok := knownModels[name]; ok {
			providerModels[name] = models
		}
	}

	convSvc := service.NewConversationService(convRepo, msgRepo, logger)
	chatSvc := service.NewChatService(convRepo, msgRepo, llmClients, providerModels, logger)

	msgHandler := handler.NewMessagesHandler(convSvc, logger)
	mux := http.NewServeMux()
	mux.Handle("/api/providers", handler.NewProvidersHandler(chatSvc, logger))
	mux.Handle("/api/chat", handler.NewChatHandler(chatSvc, logger))
	mux.Handle("/api/conversations", handler.NewConversationsHandler(convSvc, msgHandler, logger))
	mux.Handle("/api/conversations/", handler.NewConversationsHandler(convSvc, msgHandler, logger))
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	redisAddr := fmt.Sprintf("%s:%s",
		envOrDefault("REDIS_HOST", "localhost"),
		envOrDefault("REDIS_PORT", "6379"),
	)
	rdbRateLimit := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdbRateLimit.Close()

	rateLimiter := middleware.NewRateLimiter(rdbRateLimit, 20, logger)

	port := envOrDefault("CHAT_API_PORT", "4000")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      middleware.CORS(middleware.Logging(logger)(middleware.Recovery(logger)(middleware.RateLimit(rateLimiter)(mux)))),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("chat api listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
