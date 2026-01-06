$files = @(
    "pkg/types/request.go",
    "internal/streaming/parsers.go",
    "internal/streaming/forwarder.go",
    "internal/provider/openailike/openailike_test.go",
    "internal/provider/openailike/openailike.go",
    "internal/provider/openai/openai_test.go",
    "internal/provider/openai/openai.go",
    "internal/provider/gemini/gemini_test.go",
    "internal/provider/gemini/gemini.go",
    "internal/provider/cohere/cohere.go",
    "internal/provider/bedrock/bedrock.go",
    "internal/provider/azure/azure.go",
    "internal/provider/azure/azure_test.go",
    "internal/provider/anthropic/anthropic_test.go",
    "internal/provider/anthropic/anthropic.go",
    "internal/observability/otel_logs.go",
    "internal/observability/s3_callback.go",
    "internal/observability/slack_callback.go",
    "internal/observability/langfuse_callback.go",
    "internal/observability/datadog_llm_obs_callback.go",
    "internal/observability/datadog_callback.go",
    "internal/cache/handler.go",
    "internal/cache/handler_test.go",
    "internal/cache/redis.go",
    "internal/auth/postgres.go",
    "internal/api/user_endpoints.go",
    "internal/api/team_endpoints.go",
    "internal/api/organization_endpoints.go",
    "internal/api/management.go",
    "internal/api/handler.go",
    "bench/internal/runner/runner.go",
    "bench/internal/mock/server.go",
    "bench/cmd/runner/main.go"
)

foreach ($file in $files) {
    $path = Join-Path "d:\Desktop\LLMux" $file
    if (Test-Path $path) {
        $content = Get-Content $path -Raw
        if ($content -match '"encoding/json"') {
            $newContent = $content -replace '"encoding/json"', '"github.com/goccy/go-json"'
            Set-Content $path $newContent -NoNewline
            Write-Host "Updated $file"
        } else {
            Write-Host "Skipped $file (match not found)"
        }
    } else {
        Write-Host "File not found: $path"
    }
}
