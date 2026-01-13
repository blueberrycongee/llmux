#!/usr/bin/env bash
set -euo pipefail

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"

echo "=========================================="
echo "Redis Integration Test"
echo "Host: ${REDIS_HOST}:${REDIS_PORT}"
echo "=========================================="

if ! command -v redis-cli >/dev/null 2>&1; then
  echo "ERROR: redis-cli not found. Either install it, or run:"
  echo "  docker compose -f docker-compose.test.yaml exec -T redis redis-cli PING"
  exit 1
fi

echo "1. Testing Redis connection..."
redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" PING | grep -q "PONG"
echo "OK: connected"

echo ""
echo "2. Sample keys (limit 20):"
redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" --scan | head -n 20

echo ""
echo "3. Memory usage:"
redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" INFO memory | grep "^used_memory_human:" || true

echo ""
echo "=========================================="
echo "Redis test completed"
echo "=========================================="
