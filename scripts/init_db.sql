-- LLMux database bootstrap for production-like tests.
--
-- This script intentionally reuses the canonical migrations shipped in-tree,
-- instead of maintaining a separate schema copy that can drift.
--
-- Usage (repo root):
--   psql -U llmux -d llmux -f scripts/init_db.sql
--
-- Or via docker compose:
--   docker compose -f docker-compose.test.yaml exec -T postgres psql -U llmux -d llmux -f - < scripts/init_db.sql

\set ON_ERROR_STOP on

\echo 'Applying LLMux migrations...'
\i /workspace/internal/auth/migrations/001_init.sql

\echo 'Seeding test API key...'
-- "sk-test-key-12345" sha256 (hex):
--   db4aac95519f2890e4bd9f8860cceb0b452d35c5e809d69d44f234de3e7123d0
INSERT INTO api_keys (key_hash, key_prefix, name, is_active)
VALUES (
  'db4aac95519f2890e4bd9f8860cceb0b452d35c5e809d69d44f234de3e7123d0',
  'sk-test-',
  'Test API Key',
  true
)
ON CONFLICT (key_hash) DO NOTHING;

\echo 'Done.'
