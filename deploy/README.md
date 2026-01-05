# LLMux Deployment

## Quick Start

### Docker

```bash
# Build image
docker build -t llmux:latest .

# Run with environment variables
docker run -d \
  -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  llmux:latest
```

### Kubernetes (Raw Manifests)

```bash
# Create namespace and resources
kubectl apply -f k8s/namespace.yaml

# Create secrets (edit with real keys first!)
kubectl apply -f k8s/secret.yaml

# Deploy
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Verify
kubectl get pods -n llmux
```

### Helm

```bash
# Add provider secrets first
kubectl create secret generic openai-credentials \
  --from-literal=api-key=sk-xxx \
  -n llmux

# Install chart
helm install llmux ./helm/llmux \
  --namespace llmux \
  --create-namespace \
  --set providers[0].name=openai \
  --set providers[0].type=openai \
  --set providers[0].secretName=openai-credentials \
  --set providers[0].secretKey=api-key \
  --set providers[0].baseUrl=https://api.openai.com/v1 \
  --set providers[0].models={gpt-4o,gpt-4o-mini}

# Upgrade
helm upgrade llmux ./helm/llmux -n llmux

# Uninstall
helm uninstall llmux -n llmux
```

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GOOGLE_API_KEY` | Google AI API key |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API key |

### Helm Values

See `helm/llmux/values.yaml` for all available options.

Key configurations:
- `replicaCount`: Number of replicas (default: 2)
- `autoscaling.enabled`: Enable HPA (default: true)
- `config.tracing.enabled`: Enable OpenTelemetry tracing
- `providers`: List of LLM providers

## Monitoring

### Prometheus

The service exposes metrics at `/metrics`. Pod annotations enable automatic scraping:

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

### Health Checks

- Liveness: `GET /health/live`
- Readiness: `GET /health/ready`

## Security

- Runs as non-root user (65532)
- Read-only root filesystem
- No privilege escalation
- All capabilities dropped
- Distroless base image (no shell)

## Resource Recommendations

| Workload | CPU Request | CPU Limit | Memory Request | Memory Limit |
|----------|-------------|-----------|----------------|--------------|
| Light | 50m | 200m | 32Mi | 64Mi |
| Medium | 100m | 500m | 64Mi | 128Mi |
| Heavy | 200m | 1000m | 128Mi | 256Mi |
