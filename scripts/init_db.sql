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
-- "llmux_test_key_12345" sha256 (hex):
--   f0a5be3c98fccb0f2721fb33c0b8b357e93111c4399c42f591763865ae34f511
INSERT INTO api_keys (key_hash, key_prefix, name, is_active)
VALUES (
  'f0a5be3c98fccb0f2721fb33c0b8b357e93111c4399c42f591763865ae34f511',
  'llmux_te',
  'Test API Key',
  true
)
ON CONFLICT (key_hash) DO NOTHING;

\echo 'Done.'
