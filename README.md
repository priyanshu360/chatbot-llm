# LLM Inference Logging & Ingestion System

Multi-provider LLM chat with streaming responses, conversation management, and async inference log ingestion via Redis streams.

## Quick Start (Docker Compose)

```bash
cp .env.example .env
# Edit .env with your LLM API keys

docker compose -f infra/docker-compose.yml up --build
```

| Service | URL |
|---------|-----|
| Frontend | http://localhost:5173 |
| Chat API | http://localhost:4000 |
| Ingestion API | http://localhost:4001 |

## Manual Development

### Prerequisites
- Go 1.22+, Node.js 20+, Docker

### Infrastructure

```bash
docker run -d --name llm-postgres \
  -e POSTGRES_USER=llmuser -e POSTGRES_PASSWORD=llmpass -e POSTGRES_DB=llmlog \
  -p 5432:5432 postgres:16-alpine

docker run -d --name llm-redis -p 6379:6379 redis:7-alpine

cat services/db/migrations/001_init.sql | \
  docker exec -i llm-postgres psql -U llmuser -d llmlog
```

### Services

```bash
# Terminal 1: Ingestion API
cd services/ingestion-api && go run ./cmd/server

# Terminal 2: Worker
cd services/ingestion-api && go run ./cmd/worker

# Terminal 3: Chat API
cd services/chat-api && go run ./cmd/server

# Terminal 4: Frontend
cd frontend && npm install && npm run dev
```

## Kubernetes

```bash
kubectl apply -k infra/k8s/
```

Requires: Postgres Secret, LLM API Key Secret, Ingress Controller.

---

## Architecture Overview

```
Frontend (React + Vite)
    │  SSE stream  │  REST
    ▼              ▼
Chat API ──→ OpenAI / Anthropic / Gemini
    │
    └─ fire-and-forget POST ──→ Ingestion API
                                      │
                                  Redis Stream
                                      │
                                  Worker → PostgreSQL
```

### Layers

**Chat API** — SSE streaming from LLM providers, synchronous conversation/message CRUD, async inference log dispatch

**Ingestion API** — Validates incoming log payloads, publishes to Redis Stream, writes malformed payloads to DLQ

**Worker** — Reads from Redis Stream, batch-inserts valid logs to PostgreSQL, redelivers unacked messages on restart

### Project Structure

```
services/
├── chat-api/              # Go: HTTP handlers, service logic, LLM SDK wrapper
│   └── internal/
│       ├── transport/     # HTTP handlers, middleware (CORS, logging, rate limit)
│       ├── service/       # Chat orchestration, conversation CRUD, LLM clients
│       │   ├── chat.go    # SSE streaming orchestration
│       │   ├── llm/       # Provider interface + OpenAI/Anthropic/Gemini SDK
│       │   └── repo.go    # Repository interfaces
│       └── repo/          # PostgreSQL data access
├── ingestion-api/         # Go: log validation, Redis queue, batch worker
│   └── internal/
│       ├── transport/     # HTTP handlers
│       ├── service/       # Ingestion orchestration
│       ├── validation/    # Payload validation (pure)
│       └── repo/          # Queue producer/consumer + PostgreSQL
└── db/migrations/

frontend/                  # React + Vite + TypeScript
pkg/                       # Shared Go types
infra/
├── docker-compose.yml
├── docker/                # Dockerfiles
└── k8s/                   # Kubernetes manifests
```

---

## Schema Design

| Table | Purpose |
|-------|---------|
| `conversations` | Session metadata with status (`active`/`cancelled`) |
| `messages` | Full chat history with PII-safe copy |
| `inference_logs` | Per-LLM-call metadata (latency, tokens, previews) |
| `ingestion_dlq` | Dead-letter queue for malformed payloads |

### Design Decisions

- **`total_tokens` as a generated column** — PostgreSQL computes `input_tokens + output_tokens` automatically. No application-level denormalization, no stale data.
- **`seq` for ordering, not `created_at`** — avoids clock skew between distributed services. Deterministic ordering even if timestamps collide.
- **`content_redacted` alongside `content`** — PII-safe copy for analytics/export without losing the original for debugging. Access control gates the `content` column.
- **Structured columns over JSONB** — `provider`, `model`, `status` as indexed columns instead of a single JSON blob means efficient queries on any dimension. `ingestion_dlq` uses JSONB for its raw payload since its schema is unknown.
- **Indexes** — `(created_at DESC)` for time-series queries, `(provider, model)` for cost breakdowns, `(conversation_id)` for message lookups, `(conversation_id, seq)` for ordered message fetch.

---

## Key Tradeoffs

| Decision | Rationale | Alternative |
|----------|-----------|-------------|
| Async ingestion via Redis | Logging never blocks the chat hot path | Sync DB write (simpler but blocks the response) |
| In-process LLM SDK (not sidecar) | Zero network hop for provider calls | Sidecar proxy (language-agnostic but adds latency) |
| Regex PII redaction on previews | Fast, zero external dependencies | Presidio NER (more accurate but heavier, requires ML infra) |
| Separate services (not monolith) | Chat API and Ingestion API scale independently | Monolith (simpler deployment but coupling) |
| Fire-and-forget logging + buffer | Ingestion API failure never affects user experience | Synchronous write (blocks on logging) |
| DLQ for malformed payloads | Zero data loss guarantee — never silently drops | Strict validation + reject (simpler but loses data) |
| Preview truncated to 200 chars | Enough for analytics without bloating storage | Full content (more storage, no analytics benefit) |
| Custom Go validator | Zero dependency, type-safe | Zod/Pydantic (more declarative but cross-language) |

---

## What to Improve with More Time

1. **OpenTelemetry traces** — distributed tracing across Chat API → Provider → Ingestion API
2. **Presidio NER redaction** — catch names, addresses, custom entities that regex misses
3. **Cost tracking** — map token counts to per-model pricing, expose cost-per-conversation
4. **Conversation replay** — re-run conversations through different providers for A/B comparison
5. **Rate limiting per session** — per-session token budget to prevent runaway spend
6. **Table partitioning** — partition `inference_logs` by month for query performance at scale
7. **S3 archive** — move logs older than 90 days to S3 Glacier
8. **Grafana dashboard** — latency (p50/p95/p99), throughput, error rate
9. **Alerting** — PagerDuty/Slack on queue depth > 10k, error rate > 5%
10. **Auth** — API key authentication for ingestion endpoint

---

## License

MIT
