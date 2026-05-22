package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

type Logger struct {
	ingestionURL string
	client       *http.Client
	buffer       []pkg.InferenceLog
	mu           sync.Mutex
	maxBuffer    int
	ttl          time.Duration
	slog         *slog.Logger
}

func NewLogger(ingestionURL string, slog *slog.Logger) *Logger {
	l := &Logger{
		ingestionURL: ingestionURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		},
		buffer:    make([]pkg.InferenceLog, 0, 100),
		maxBuffer: 100,
		ttl:       5 * time.Minute,
		slog:      slog,
	}

	go l.flushLoop()
	return l
}

func (l *Logger) Log(log pkg.InferenceLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buffer) < l.maxBuffer {
		l.buffer = append(l.buffer, log)
	}
}

func (l *Logger) flushLoop() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		l.flush()
		l.evictStale()
	}
}

func (l *Logger) flush() {
	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return
	}
	batch := make([]pkg.InferenceLog, len(l.buffer))
	copy(batch, l.buffer)
	l.buffer = l.buffer[:0]
	l.mu.Unlock()

	l.slog.Debug("flushing inference logs", "count", len(batch))

	for i, log := range batch {
		if err := l.send(log); err != nil {
			l.mu.Lock()
			if len(l.buffer) < l.maxBuffer {
				l.buffer = append(l.buffer, log)
			}
			l.mu.Unlock()
			if i < len(batch)-1 {
				l.slog.Debug("inference log send failed, will retry", "error", err)
			}
		}
	}
}

func (l *Logger) evictStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.ttl)
	var kept []pkg.InferenceLog
	for _, log := range l.buffer {
		ts, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil || ts.After(cutoff) {
			kept = append(kept, log)
		}
	}
	if n := len(l.buffer) - len(kept); n > 0 {
		l.slog.Warn("evicted stale inference logs", "count", n)
	}
	l.buffer = kept
}

func (l *Logger) send(log pkg.InferenceLog) error {
	body, err := json.Marshal(log)
	if err != nil {
		return err
	}

	l.slog.Debug("sending inference log", "request_id", log.RequestID, "provider", log.Provider, "latency_ms", log.LatencyMs)
	resp, err := l.client.Post(l.ingestionURL+"/v1/ingest/inference", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("ingestion api returned %d", resp.StatusCode)
	}
	return nil
}
