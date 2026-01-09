#!/bin/bash
# Script to add SupportsEmbedding: false to remaining openailike providers
# This is a conservative approach - providers should explicitly opt-in to embedding support

PROVIDERS=(
    "ai21"
    "anyscale"
    "baichuan"
    "cerebras"
    "cloudflare"
    "cohere"
    "databricks"
    "deepinfra"
    "fireworks"
    "github"
    "huggingface"
    "hunyuan"
    "hyperbolic"
    "lambda"
    "lmstudio"
    "minimax"
    "mistral"
    "moonshot"
    "novita"
    "nvidia"
    "openrouter"
    "perplexity"
    "qwen"
    "replicate"
    "sambanova"
    "siliconflow"
    "snowflake"
    "stepfun"
    "volcengine"
    "watsonx"
    "xai"
    "yi"
    "zhipu"
)

for provider in "${PROVIDERS[@]}"; do
    file="d:/Desktop/LLMux/providers/${provider}/${provider}.go"
    if [ -f "$file" ]; then
        # Check if SupportsEmbedding is already set
        if ! grep -q "SupportsEmbedding:" "$file"; then
            echo "Updating $provider..."
            # Add SupportsEmbedding: false after SupportsStreaming line
            sed -i 's/\(SupportsStreaming: [^,]*,\)/\1\n\tSupportsEmbedding: false, \/\/ Default: explicitly disabled/' "$file"
        else
            echo "Skipping $provider (already configured)"
        fi
    else
        echo "Warning: $file not found"
    fi
done

echo "Done!"
