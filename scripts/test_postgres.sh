#!/usr/bin/env bash
set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-llmux}"
DB_NAME="${DB_NAME:-llmux}"
DB_PASSWORD="${DB_PASSWORD:-}"

echo "=========================================="
echo "PostgreSQL Integration Test"
echo "Host: ${DB_HOST}:${DB_PORT}"
echo "DB: ${DB_NAME}  User: ${DB_USER}"
echo "=========================================="

if ! command -v psql >/dev/null 2>&1; then
  echo "ERROR: psql not found. Either install it, or run:"
  echo "  docker compose -f docker-compose.test.yaml exec -T postgres psql -U llmux -d llmux -c 'SELECT 1'"
  exit 1
fi

export PGPASSWORD="${DB_PASSWORD}"
PSQL=(psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}")

echo "1. Testing PostgreSQL connection..."
"${PSQL[@]}" -c "SELECT 1" >/dev/null
echo "OK: connected"

echo ""
echo "2. Tables (top-level):"
"${PSQL[@]}" -c "\\dt"

echo ""
echo "3. API keys:"
"${PSQL[@]}" -c "SELECT COUNT(*) AS total, COUNT(*) FILTER (WHERE is_active) AS active FROM api_keys;"

echo ""
echo "4. Usage logs (last 24h):"
"${PSQL[@]}" -c "SELECT COUNT(*) AS requests, COALESCE(SUM(total_tokens), 0) AS total_tokens FROM usage_logs WHERE created_at > NOW() - INTERVAL '24 hours';"

echo ""
echo "=========================================="
echo "PostgreSQL test completed"
echo "=========================================="
