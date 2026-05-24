# Architecture Notes

## Ingestion Flow

```
Chat API                     Ingestion API              Redis                    Worker                 PostgreSQL
   │                              │                       │                        │                       │
   │  POST /v1/ingest/inference   │                       │                        │                       │
   │─────────────────────────────▶│                       │                        │                       │
   │                              │  Validate payload     │                        │                       │
   │                              │    │ valid            │                        │                       │
   │                              │    ├──▶ XAdd stream   │                        │                       │
   │                              │    │     ────────────▶│                        │                       │
   │                              │    │                  │  XReadGroup (blocking)  │                       │
   │                              │    │                  │◀────────────────────────│                       │
   │                              │    │                  │                        │                       │
   │  202 Accepted ◀──────────────│    │                  │                        │                       │
   │                              │    │                  │                        │                       │
   │                              │    │ invalid          │                        │                       │
   │                              │    └──▶ INSERT dlq ──▶│                        │                       │
   │                              │                       │                        │  Batch INSERT         │
   │                              │                       │                        │──────────────────────▶│
   │                              │                       │                        │  XAck                 │
   │                              │                       │◀────────────────────────│                       │
```

Four components, one direction:

1. **Chat API** dispatches inference logs via fire-and-forget HTTP POST to the Ingestion API
2. **Ingestion API** validates the payload, then either publishes to Redis Stream or writes to the DLQ
3. **Worker** reads from Redis Stream in batches (500 messages / 2s flush), separates valid rows from invalid, batch-inserts to PostgreSQL, then ACKs the Redis messages
4. **Failure path**: if the batch insert fails, the worker does NOT ACK — messages remain in Redis for redelivery

---

## Logging Strategy

### What Gets Logged

Every LLM inference call produces one row in `inference_logs`:

| Field | Source | Purpose |
|-------|--------|---------|
| `request_id` | UUID generated per call | Deduplication (`ON CONFLICT DO NOTHING`) |
| `session_id` | Conversation ID | Session grouping for analytics |
| `conversation_id` | Conversation ID | Conversation lookup |
| `message_id` | User message UUID | Links inference back to the user message |
| `provider`, `model` | Chat request | Cost & usage breakdown |
| `latency_ms` | Wall clock | Performance monitoring |
| `input_tokens`, `output_tokens` | Provider response | Token accounting |
| `status` | `success` / `error` / `timeout` | Error rate tracking |
| `error_code` | Provider error message | Structured error diagnosis (e.g. `deadline_exceeded`, `api error: status 404`) |
| `input_preview`, `output_preview` | First 200 chars, PII-redacted | Debug samples without exposing sensitive data |
| `timestamp` | `time.Now().UTC().RFC3339` | When the inference was made |

### Data Flow

```
LLM call completes
    │
    ▼
Capture metadata (tokens, latency, preview, messageID)
    │
    ▼
Redact PII from previews (regex: email, phone, CC, IP, passport)
    │
    ▼
Check context for deadline exceeded → status = "timeout" / error_code = "deadline_exceeded"
    │
    ▼
BufferedLogger (in-memory buffer, max 100, TTL 5 min)
    │
    ▼ every 5s or on buffer full
POST /v1/ingest/inference  ──→ Ingestion API
    │
    ▼ success
Drop from buffer
    │
    ▼ failure (ingestion API down)
Retry on next flush
    │
    ▼ TTL exceeded (>5 min in buffer)
Evict (lost — acceptable for non-critical analytics)
```

### Design Decisions

- **Fire-and-forget, not synchronous**: Logging should never add latency to the chat response. The `BufferedLogger` accepts writes instantly, batches, and retries on failure.
- **Buffered, not streaming**: The buffer absorbs transient ingestion API failures. 100-entry capacity with 5-minute TTL means the chat API survives a 5-minute ingestion outage without losing a single log event.
- **PII redaction at source**: Previews are redacted in the Chat API before entering the buffer. The Ingestion API and database never see raw PII in the preview fields.
- **Separate `content` and `content_redacted`**: The `messages` table stores both the original and the redacted copy. This lets you run analytics on the redacted copy while preserving the original for debugging (access-controlled).

---

## Scaling Considerations

### Write Path (Inference Logs)

| Component | Scaling Strategy | Bottleneck |
|-----------|-----------------|------------|
| Ingestion API | Stateless HTTP — scale horizontally behind a service | Redis throughput (~50k ops/sec per node) |
| Redis Stream | Single node today — next step: Redis Cluster with partitioning by `provider` | Stream memory (~500 MB at 50k logs/min with 200B previews) |
| Worker | Stateless — scale horizontally. Each pod gets a unique consumer name via `os.Hostname()` | PostgreSQL write throughput (~5k rows/sec on a single node) |
| PostgreSQL | Batch inserts amortize overhead. Next step: partition `inference_logs` by month | Disk I/O on sequential writes |

Current tuning:
- Worker batch size: **500** messages
- Flush interval: **2 seconds**
- Redis stream `MAXLEN`: **50,000** (approx 10 minutes of buffer at peak)
- pgxpool `MaxConns`: **25** (chat-api), **15** (ingestion-api), **10** (worker)

### Read Path (Chat API)

- Conversations and messages are read directly from PostgreSQL via indexed queries
- Read volume is low (one read per chat request) — at least 10x smaller than write volume
- If read pressure grows, add a Redis cache layer for conversation/message data with TTL-based invalidation

### Streaming (SSE)

- Each SSE connection holds an open HTTP connection to the Chat API
- Long-running connections (minutes per conversation turn)
- Nginx ingress configured with `proxy-buffering: off` and `proxy-read-timeout: 300s`
- Rate limiter: 20 req/s per IP via Redis token bucket (shared across pod replicas)

### Rate Limiting

- **20 requests per second per IP** using Redis `INCR` with a 1-second window
- Implemented as a `middleware.RateLimiter` using an atomic Lua script
- Redis-based (not in-memory) so limits are shared across all Chat API pod replicas
- Returns `429 Too Many Requests` with `Retry-After: 1` header
- Only applied to Chat API (Ingestion API is internal)

---

---

## Context Management

### Strategy

Each LLM call truncates historical messages to fit within the model's context window, reserving 20% for the response. Messages are dropped oldest-first, preserving the most recent turns.

### Token Estimation

A fast heuristic (`len(text) / 4`) estimates token counts without a tokenizer dependency. This ~4x ratio is typical for English text and avoids the cost and latency of calling per-model tokenizers on every turn.

### Model Context Windows

Defined in `services/chat-api/internal/service/modelinfo.go`:

| Provider | Model | Context Window |
|----------|-------|---------------|
| openai | gpt-4o | 128,000 |
| openai | gpt-4o-mini | 128,000 |
| anthropic | claude-sonnet-4-6 | 200,000 |
| anthropic | claude-haiku-4-5 | 200,000 |
| gemini | gemini-2.5-flash | 1,000,000 |
| gemini | gemini-1.5-flash | 1,000,000 |
| ollama | llama3.2 | 128,000 |
| ollama | llama3.1 | 128,000 |
| ollama | mistral | 32,000 |
| ollama | phi4 | 128,000 |
| deepseek | deepseek-chat | 65,536 |
| deepseek | deepseek-reasoner | 65,536 |

Unknown models default to 32,000 as a safe fallback.

### Algorithm

In `StreamChat` (`service/chat.go:98-113`):

1. Compute `budget = contextWindow(provider, model) * 0.8`
2. Walk `chatHistory` from newest to oldest
3. Always keep the current user message (last item)
4. Keep older messages while `remaining budget >= message tokens`
5. Drop the rest (oldest messages exceed budget)

This is message-count-agnostic — it naturally keeps more short messages (rapid Q&A) and fewer long messages (large code proofs).

### Future Improvements

- Replace heuristic token estimation with per-model tokenizers for precise counts
- Add summarization of dropped messages instead of dropping entirely
- Make the budget ratio configurable per-model in the lookup table
- Cache token counts per message to avoid re-estimating on every turn

---

## Failure Handling Assumptions

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| **Ingestion API down** | Chat API's `BufferedLogger` holds up to 100 events, retries every 5s. Events older than 5 min are dropped | Logging is non-critical analytics — losing a few events is acceptable if the ingestion API is down for extended periods |
| **Worker crash** | Redis consumer group does NOT ACK in-flight messages. On restart, the worker re-receives unacked messages | At-least-once delivery for inference logs. A brief duplicate window is acceptable |
| **Worker scale-up** | Each new worker pod uses its hostname as the consumer name. Multiple workers share the same consumer group, each getting a partition of messages | Redis Streams automatically balances messages across consumers in the same group |
| **Malformed payload** | Ingestion API writes the raw JSON + validation error to `ingestion_dlq` and returns `422`. The log is never silently dropped | Zero data loss for debugging feed issues |
| **PostgreSQL slow/full** | Worker pauses consumption. Messages accumulate in Redis stream. Stream hits `MAXLEN` 50k and starts evicting oldest | Backpressure: Redis acts as a shock absorber. Alert when stream length > 10k |
| **PostgreSQL connection pool exhausted** | Pool waits for a connection (configurable `MaxConns`). Requests queue at the pool level, not HTTP level | `pgxpool` handles this gracefully — no connection storms |
| **LLM provider timeout** | The `Provider.StreamChat` context is cancelled. The SDK captures `status: "timeout"` with `error_code: "deadline_exceeded"`, logs it, and returns the error to the UI | Partial results are not stored — only the error code |
| **Client disconnect** | `request.Context()` is cancelled. The Chat API goroutine detects it and tears down the provider stream. No orphaned provider calls | Context propagation ensures cleanup even if the client navigates away mid-stream |
| **Redis unavailable** | Rate limiter fails open (allows the request). Existing rate limits are temporarily ineffective | Rate limiting is a defense-in-depth measure, not a hard security boundary |
| **Duplicate inference logs** | `inference_logs` has a `UNIQUE` constraint on `request_id`. Duplicate inserts are silently skipped | At-most-once for storage, at-least-once for queuing — deduped at the DB level |

---

## API Endpoints

### Chat API (:4000)

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/providers | List available providers and their models |
| POST | /api/chat | Send message, returns SSE stream of tokens |
| POST | /api/conversations | Create conversation |
| GET | /api/conversations | List conversations (last 100, ordered by `updated_at`) |
| GET | /api/conversations/{id} | Get conversation metadata |
| GET | /api/conversations/{id}/messages | Get all messages for a conversation |
| PATCH | /api/conversations/{id} | Cancel (`{"status":"cancelled"}`) or resume (`{"status":"active"}`) |
| DELETE | /api/conversations/{id} | Delete conversation and all messages (CASCADE) |

Errors are returned as structured JSON with the `error` key and a `field` key for validation errors:

```json
{"error": "conversation not found", "status": 404}
{"error": "provider is required", "field": "provider", "status": 400}
```

### Ingestion API (:4001)

| Method | Path | Description |
|--------|------|-------------|
| POST | /v1/ingest/inference | Submit an inference log payload |
| GET | /v1/health | Health check |

Validation errors return `422` with per-field details:

```json
{"accepted": 0, "errors": [{"field": "request_id", "message": "request_id is required"}]}
```

Queue unavailable returns `503`:

```json
{"error": "queue unavailable"}
```

---

## Database Choice for Ingested Logs

The `inference_logs` table stores every LLM inference event and serves as the data
source for Grafana dashboards (latency, throughput, error rate). Here is why
PostgreSQL was chosen and how alternatives compare:

| Choice | Rationale |
|--------|-----------|
| **PostgreSQL** (chosen) | Already the application database — zero additional infrastructure. Rich SQL with percentile (`percentile_cont`), window functions, and CTEs. Grafana's PostgreSQL datasource provides full query capability. Keeps the stack simple: one operational DB for both transactions and analytics. |
| **TimescaleDB** (would use given more time) | Native time-series partitioning (automatic chunking by `created_at`), continuous aggregates for pre-rolled metrics, full SQL compatibility. If we had more time, this would be the upgrade path. Hypertables on `created_at` would speed up range queries significantly at scale and reduce storage via compression policies. |
| **ClickHouse** | Column-oriented OLAP database with blistering-fast aggregate queries. Overkill for current volume (~50k logs/min). Requires separate operational footprint (ZooKeeper/Keeper for replication, merge trees for compaction). Different wire protocol and query dialect. |
| **InfluxDB** | Purpose-built time-series DB with Flux/InfluxQL. Strong write throughput but different query language than SQL means a separate skillset. Grafana support is good but query patterns don't compose naturally with relational data (e.g., joining with `conversations` for context). |
| **MongoDB** | Document store with a time-series collection feature. Poor Grafana support — the Grafana MongoDB datasource is community-maintained and lacks the polished SQL/PromQL experience. No native support for percentile aggregations; requires aggregation pipelines for what PostgreSQL does with `percentile_cont`. |
| **Elasticsearch** | Excellent for full-text search on `input_preview` / `output_preview` (e.g., "find all logs where the user asked about X"). However, for numeric time-series metrics (latency, tokens, counts), PostgreSQL is simpler and more efficient. A future hybrid: Elasticsearch for text search, PostgreSQL for metrics, correlated via `request_id`. |

### Decision rationale

**Keep it simple.** PostgreSQL was already running, the team knew it, and Grafana
queries it via standard SQL. At the current scale (~50k logs/min, ~500GB/year
before archival), a single PostgreSQL instance handles both transactional reads
(conversations/messages) and analytical queries (metrics) without issue. When
the analytical workload outgrows the transactional instance, we can:

1. Set up a read-replica dedicated to Grafana queries
2. Migrate to TimescaleDB hypertables for automatic time-based partitioning
3. Offload historical data to ClickHouse for long-range analytics

All three upgrade paths preserve the existing SQL queries with minimal changes.

---

## Database Tables

### conversations

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK, `gen_random_uuid()` |
| title | TEXT | User-facing title |
| provider | TEXT | `openai` / `anthropic` / `gemini` / `ollama` / `deepseek` |
| model | TEXT | e.g. `gpt-4o`, `claude-sonnet-4` |
| status | TEXT | `active` / `cancelled`, CHECK constraint |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | Updated on message |

### messages

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| conversation_id | UUID | FK → conversations, ON DELETE CASCADE |
| role | TEXT | `user` / `assistant` / `system` |
| content | TEXT | Full message (access-controlled for PII) |
| content_redacted | TEXT | PII-scrubbed copy (safe for analytics) |
| seq | INT | Deterministic ordering within conversation |
| created_at | TIMESTAMPTZ | |

### inference_logs

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| request_id | UUID | UNIQUE — deduplicates retried deliveries |
| conversation_id | UUID | |
| message_id | UUID | |
| provider | TEXT | |
| model | TEXT | |
| latency_ms | INT | Wall clock in milliseconds |
| input_tokens | INT | From provider response |
| output_tokens | INT | From provider response |
| total_tokens | INT | GENERATED ALWAYS AS (input + output) |
| status | TEXT | `success` / `error` / `timeout` |
| error_code | TEXT | e.g. `rate_limit_exceeded`, `timeout` |
| input_preview | TEXT | First 200 chars, PII-redacted |
| output_preview | TEXT | First 200 chars, PII-redacted |
| created_at | TIMESTAMPTZ | |

### ingestion_dlq

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| raw | JSONB | Full original payload (preserved for replay) |
| error_msg | TEXT | Human-readable validation error |
| created_at | TIMESTAMPTZ | |

---

## Future Improvements

### Frontend Error Handling

Add a React error boundary, centralized error state in the Zustand store, a toast notification system, surface streaming/mutation errors to the UI, fix silent `.catch()` swallows, and add network connectivity detection.

### Grafana Dashboard Metrics

Add DLQ monitoring panels (`ingestion_dlq` table), provider breakdown panels (latency/error/token by provider), and cost estimation panels (per-model pricing lookup via SQL `CASE`).

### Elasticsearch Full-Text Search

Index `input_preview` and `output_preview` in Elasticsearch for fast search across inference logs, correlated to the PostgreSQL data via `request_id`.

### Auth & Usage Management

Add user authentication (OAuth2/OIDC or API keys) with per-user rate limits and token budgets. Track per-user spend against tiered pricing plans; expose usage dashboards and enforcement via the ingestion pipeline.
