# Test Checklist

No tests exist yet. This checklist defines what to write, in priority order.

## Go Backend Tests

### P0 — Core Logic (pure functions, no deps)

| Test | Package | What to cover |
|------|---------|---------------|
| PII redaction | `chat-api/service/llm` | Email, phone, credit card, IP, passport patterns. Redacted vs original. Edge cases: short input, no match, all PII. |
| Title truncation | `chat-api/util` | Empty string, exactly 50 chars, >50 chars, unicode. |
| Message truncation | `chat-api/service/llm` | String <200, =200, >200, empty. |
| Validation rules | `ingestion-api/validation` | Each required field missing, each enum value, negative ints, bad timestamp format, valid payload passes. |

### P1 — Service Layer (mock repos)

| Test | Package | What to cover |
|------|---------|---------------|
| ConversationService.Create | `chat-api/service` | Empty provider → `InputError`, empty model → `InputError`, valid → calls repo. |
| ConversationService.Get | `chat-api/service` | Repo returns `ErrNotFound` → `ErrConversationNotFound`. Repo returns record → success. |
| ConversationService.Cancel | `chat-api/service` | Repo `ErrNotFound` → `ErrConversationNotFound`. Success → no error. |
| ConversationService.Resume | `chat-api/service` | Same pattern as Cancel. |
| ConversationService.Delete | `chat-api/service` | Same pattern as Cancel. |
| ChatService.StreamChat | `chat-api/service` | Unknown provider → `InputError`. Cancelled conversation → `ErrConversationCancelled`. Empty message → `InputError`. |
| IngestService.ProcessLog | `ingestion-api/service` | Valid payload → publishes + returns Accepted=1. Invalid payload → writes to DLQ + returns errors. Queue down → `ErrQueueUnavailable`. |
| Error wrapping | `chat-api/service` | `errors.Is(err, repo.ErrNotFound)` → `ErrConversationNotFound` for each CRUD method. |

### P2 — Transport Handlers (httptest)

| Test | Package | What to cover |
|------|---------|---------------|
| ChatHandler 400 | `chat-api/transport/handler` | Missing body, invalid JSON, empty message, unknown provider. |
| ChatHandler 404 | `chat-api/transport/handler` | Non-existent conversation ID. |
| ChatHandler 200 | `chat-api/transport/handler` | Valid request returns `text/event-stream` with `event: meta`. |
| ConversationsHandler CRUD | `chat-api/transport/handler` | GET list, GET by ID (found/not found), POST create (valid/invalid), PATCH cancel/resume, DELETE. |
| MessagesHandler | `chat-api/transport/handler` | GET messages for conversation. |
| IngestHandler 202 | `ingestion-api/transport/handler` | Valid payload → 202 Accepted. |
| IngestHandler 422 | `ingestion-api/transport/handler` | Invalid payload → 422 with field errors. |
| IngestHandler 503 | `ingestion-api/transport/handler` | Queue unavailable → 503. |
| Error body format | `chat-api/transport/handler` | `writeError` returns correct status + JSON body for each error type. |

### P3 — Repository Layer (testcontainers)

| Test | Package | What to cover |
|------|---------|---------------|
| ConversationRepo CRUD | `chat-api/repo` | Create, GetByID (found/not found), List, UpdateStatus, Delete, Touch. |
| ConversationRepo not found | `chat-api/repo` | `GetByID("nonexistent")` → `ErrNotFound`. `UpdateStatus` on missing → `ErrNotFound`. `Delete` on missing → `ErrNotFound`. |
| MessageRepo | `chat-api/repo` | Insert, GetByConversation, NextSeq. |
| InferenceLogRepo batch | `ingestion-api/repo` | BatchInsert, duplicate `request_id` is skipped. |
| DLQRepo | `ingestion-api/repo` | Insert, read back. |
| Queue producer/consumer | `ingestion-api/repo` | Publish, ReadBatch, Ack (needs Redis container). Consumer group creation idempotent. |

## Frontend Tests

### P0 — Hooks & Stores (no DOM)

| Test | What to cover |
|------|---------------|
| `useChatStream` | SSE event parsing: `event: token`, `event: done`, `event: error`, `event: meta`. Connection lifecycle: open → streaming → close. Error on HTTP failure. |
| Zustand conversation store | Create conversation, select conversation, update messages, cancel. |
| `api.ts` HTTP client | Request formatting, response parsing, error handling for 400/404/500. |

### P1 — Components (React Testing Library)

| Test | What to cover |
|------|---------------|
| ChatWindow | Renders messages from store. Shows loading state. Shows error state. Empty state. |
| ChatInput | Submit on Enter. Disabled while streaming. Disabled when empty. |
| Sidebar | Lists conversations. Click to select. Shows active indicator. |
| ProviderPicker | Switch provider. Shows current selection. |

### P2 — Integration (mocked API)

| Test | What to cover |
|------|---------------|
| Send message → receive stream | Mock SSE → assert messages appear in UI. |
| Cancel conversation → disabled input | PATCH cancel → input disabled. |
| Create conversation → sidebar updates | POST create → sidebar shows new item. |

## Manual / E2E Test Scenarios

These can be run against a local dev environment.

### Chat

- [ ] Send a message and verify SSE token-by-token rendering
- [ ] Send a second message in the same conversation — verify context is preserved
- [ ] Create a new conversation — verify it appears in the sidebar
- [ ] Switch between conversations — verify messages are correct per conversation
- [ ] Cancel a conversation — verify "cancelled" status, verify new messages are rejected with proper error
- [ ] Resume a cancelled conversation — verify it works again
- [ ] Delete a conversation — verify it disappears from sidebar
- [ ] Switch provider mid-conversation — verify the next message uses the new provider

### Edge Cases

- [ ] Send an empty message — verify 400 error in UI
- [ ] Send a message with PII (email, phone, credit card) — verify preview in `inference_logs` is redacted but `content` is not
- [ ] Send a very long message (>5000 chars) — verify it's handled
- [ ] Rapidly send messages (click send 10x) — verify only the first goes through (rate limiter)
- [ ] Disconnect network mid-stream — verify UI shows error
- [ ] Reconnect and continue conversation — verify history loads

### Infrastructure

- [ ] Stop ingestion-api — verify chat still works, logs buffer, logs appear when ingestion-api comes back
- [ ] Stop worker — verify Redis stream grows, logs appear when worker restarts
- [ ] Kill worker pod (K8s) — verify new worker pod picks up unacked messages
- [ ] Scale chat-api to 3 replicas — verify rate limiter works across pods (shared Redis state)
- [ ] Scale worker to 3 replicas — verify each has unique consumer name, messages are distributed
- [ ] Fill Redis stream past MAXLEN (50k) — verify oldest messages are evicted gracefully

### Database

- [ ] Insert duplicate `request_id` — verify `ON CONFLICT DO NOTHING` silently skips
- [ ] Delete a conversation — verify messages CASCADE delete
- [ ] Query `inference_logs` by provider — verify index is used (EXPLAIN ANALYZE)
- [ ] Query `total_tokens` — verify generated column is computed correctly (input + output)

## Test Infrastructure Setup

```bash
# Go tests — stdlib testing, no framework needed
cd services/chat-api && go test ./...

# With testcontainers for Postgres/Redis integration tests:
cd services/ingestion-api && go test -tags=integration ./...

# Frontend — add vitest
cd frontend && npm install -D vitest @testing-library/react @testing-library/jest-dom
```

## Priority Matrix

| Priority | Layer | Why |
|----------|-------|-----|
| P0 | Pure functions | Fast, no deps, highest ROI — catch regressions in PII and validation |
| P0 | Service layer with mocks | Core business logic — catch error-type mismatches |
| P1 | Transport handlers | HTTP contract — catch wrong status codes and response bodies |
| P1 | Hooks & stores | Frontend data flow — catch SSE parsing bugs |
| P2 | Components | UI rendering — lower ROI, slower tests |
| P3 | Repository with testcontainers | Slowest, heaviest — verify SQL correctness and edge cases |
