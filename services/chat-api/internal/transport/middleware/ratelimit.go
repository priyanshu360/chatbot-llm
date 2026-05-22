package middleware

import (
	"context"
	_ "embed"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

//go:embed ratelimit.lua
var rateLimitScript string

type RateLimiter struct {
	rdb    *redis.Client
	rate   int
	window time.Duration
	prefix string
	logger *slog.Logger
}

func NewRateLimiter(rdb *redis.Client, ratePerSec int, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		rate:   ratePerSec,
		window: time.Second,
		prefix: "ratelimit",
		logger: logger,
	}
}

func (rl *RateLimiter) allow(ctx context.Context, ip string) bool {
	now := timeNow()
	key := strings.Join([]string{rl.prefix, ip, strconv.FormatInt(now.Unix(), 10)}, ":")

	allowed, err := rl.rdb.Eval(ctx, rateLimitScript, []string{key}, rl.rate, int(rl.window.Seconds())).Bool()
	if err != nil {
		rl.logger.Error("rate limit eval failed", "error", err)
		return true
	}
	return allowed
}

var timeNow = time.Now

func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}

			if !rl.allow(r.Context(), host) {
				rl.logger.Warn("rate limit exceeded", "ip", host, "path", r.URL.Path)
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
