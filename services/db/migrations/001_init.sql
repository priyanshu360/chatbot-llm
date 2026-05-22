CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE conversations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title       TEXT NOT NULL DEFAULT '',
  provider    TEXT NOT NULL,
  model       TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE messages (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
  content         TEXT NOT NULL,
  content_redacted TEXT,
  seq             INT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inference_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id      UUID NOT NULL UNIQUE,
  conversation_id UUID,
  message_id      UUID,
  provider        TEXT NOT NULL,
  model           TEXT NOT NULL,
  latency_ms      INT NOT NULL,
  input_tokens    INT,
  output_tokens   INT,
  total_tokens    INT GENERATED ALWAYS AS (COALESCE(input_tokens,0) + COALESCE(output_tokens,0)) STORED,
  status          TEXT NOT NULL CHECK (status IN ('success', 'error', 'timeout')),
  error_code      TEXT,
  input_preview   TEXT,
  output_preview  TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingestion_dlq (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  raw        JSONB NOT NULL,
  error_msg  TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_inference_logs_created_at ON inference_logs (created_at DESC);
CREATE INDEX idx_inference_logs_provider_model ON inference_logs (provider, model);
CREATE INDEX idx_inference_logs_conversation ON inference_logs (conversation_id);
CREATE INDEX idx_messages_conversation_seq ON messages (conversation_id, seq);
CREATE INDEX idx_conversations_updated_at ON conversations (updated_at DESC);
