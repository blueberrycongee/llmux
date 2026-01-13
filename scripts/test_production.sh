#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_KEY="${TEST_API_KEY:-llmux_test_key_12345}"

echo "=========================================="
echo "LLMux Production Test Suite"
echo "Base URL: ${BASE_URL}"
echo "=========================================="

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; exit 1; }
info() { echo "INFO: $1"; }

auth_header() {
  printf "Authorization: Bearer %s" "${API_KEY}"
}

json() {
  curl -sf "${@}"
}

echo ""
echo "--- 1. Health Checks ---"
info "GET /health/live"
json "${BASE_URL}/health/live" >/dev/null && pass "Liveness probe" || fail "Liveness probe"
info "GET /health/ready"
json "${BASE_URL}/health/ready" >/dev/null && pass "Readiness probe" || fail "Readiness probe"

echo ""
echo "--- 2. Metrics ---"
info "GET /metrics"
METRICS="$(json "${BASE_URL}/metrics")"
echo "${METRICS}" | grep -q "^llmux_proxy_total_requests" && pass "Metrics endpoint" || fail "Metrics endpoint"

echo ""
echo "--- 3. Chat Completions (Non-Streaming) ---"
info "POST /v1/chat/completions"
RESPONSE="$(json "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "$(auth_header)" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hello in one word"}],
    "max_tokens": 10
  }')"
echo "${RESPONSE}" | grep -q "\"choices\"" && pass "Chat completion" || fail "Chat completion"

echo ""
echo "--- 4. Chat Completions (Streaming) ---"
info "POST /v1/chat/completions (stream=true)"
curl -sfN "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "$(auth_header)" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Count 1 to 3"}],
    "stream": true,
    "max_tokens": 20
  }' | head -n 30 | grep -q "^data:" && pass "Streaming response" || fail "Streaming response"

echo ""
echo "--- 5. Models List ---"
info "GET /v1/models"
MODELS="$(json "${BASE_URL}/v1/models" -H "$(auth_header)")"
echo "${MODELS}" | grep -q "\"data\"" && pass "Models list" || fail "Models list"

echo ""
echo "--- 6. Cache (best effort) ---"
info "Sending 2 identical requests and checking cache metrics (if enabled)"
json "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "$(auth_header)" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "max_tokens": 5
  }' >/dev/null || true
json "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "$(auth_header)" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "max_tokens": 5
  }' >/dev/null || true
CACHE_METRICS="$(json "${BASE_URL}/metrics" | grep "^llmux_cache_hits_total" || true)"
if [ -n "${CACHE_METRICS}" ]; then
  pass "Cache metrics present (hits may still be 0 depending on config)"
else
  info "Cache metrics not present (cache plugin may be disabled)"
fi

echo ""
echo "--- 7. Error Handling ---"
info "Invalid model should return non-200"
HTTP_CODE="$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "$(auth_header)" \
  -d '{
    "model": "non-existent-model",
    "messages": [{"role": "user", "content": "test"}]
  }')"
[ "${HTTP_CODE}" != "200" ] && pass "Invalid model rejected (${HTTP_CODE})" || fail "Should reject invalid model"

echo ""
echo "--- 8. Concurrent Requests (best effort) ---"
info "Sending 5 concurrent requests"
for i in 1 2 3 4 5; do
  curl -sf "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "$(auth_header)" \
    -d "{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"Say ${i}\"}], \"max_tokens\": 5}" >/dev/null &
done
wait
pass "Concurrent requests completed"

echo ""
echo "=========================================="
echo "All tests completed."
echo "=========================================="
