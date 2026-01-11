# LLMux 中间件部署指南

本文档详细介绍了如何在云服务器上使用 K3s 部署 LLMux 所需的中间件（Redis, PostgreSQL, Qdrant）。

## 1. 架构概览

为了支持 LLMux 的企业级功能（持久化、限流、向量记忆），我们需要部署以下服务：

| 服务 | 版本 | 用途 |
| :--- | :--- | :--- |
| **PostgreSQL** | latest (pgvector) | 用户认证、API Key 管理、审计日志、(可选) 向量存储 |
| **Redis** | alpine | 分布式限流、缓存 |
| **Qdrant** | latest | 记忆系统向量存储 (Vector DB) |

## 2. 部署方式 A: Docker Compose (单机简单部署)

如果您不想使用 Kubernetes，可以直接使用 Docker Compose。

**配置文件**: [`docker-compose.middleware.yml`](./docker-compose.middleware.yml)

**启动命令**:
```bash
docker compose -f docker-compose.middleware.yml up -d
```

---

## 3. 部署方式 B: K3s (推荐 - 轻量级 Kubernetes)

K3s 是一个轻量级的 Kubernetes 发行版，适合单节点云服务器，且方便未来扩展。

### 3.1 安装 K3s

在云服务器上执行：
```bash
curl -sfL https://get.k3s.io | sh -
```
验证安装：
```bash
sudo kubectl get nodes
```

### 3.2 配置国内镜像加速 (关键)

由于网络原因，直接拉取 Docker Hub 镜像可能会超时。请配置 containerd 镜像加速：

1. 编辑/创建 `/etc/rancher/k3s/registries.yaml`:
   ```yaml
   mirrors:
     docker.io:
       endpoint:
         - "https://docker.m.daocloud.io"
         - "https://huecker.io"
         - "https://mirror.baidubce.com"
   ```
2. 重启 K3s:
   ```bash
   sudo systemctl restart k3s
   ```

### 3.3 部署中间件

**配置文件**: [`k3s-middleware.yaml`](./k3s-middleware.yaml)

1. 上传文件到服务器。
2. 应用配置：
   ```bash
   sudo kubectl apply -f k3s-middleware.yaml
   ```
3. 验证状态：
   ```bash
   sudo kubectl get pods -n llmux-middleware -w
   ```
   等待所有 Pod 状态变为 `Running`。

## 4. 验证与测试

部署完成后，建议运行以下测试以确保服务可用。

### 4.1 Redis 测试
```bash
# 获取 Pod 名称
REDIS_POD=$(kubectl get pod -n llmux-middleware -l app=redis -o jsonpath="{.items[0].metadata.name}")

# 进入容器测试
kubectl exec -it -n llmux-middleware $REDIS_POD -- sh -c "redis-cli -a changeme_in_production PING"
# 预期输出: PONG
```

### 4.2 PostgreSQL (pgvector) 测试
```bash
# 获取 Pod 名称
PG_POD=$(kubectl get pod -n llmux-middleware -l app=postgres -o jsonpath="{.items[0].metadata.name}")

# 验证 pgvector 扩展
kubectl exec -it -n llmux-middleware $PG_POD -- bash -c "psql -U llmux -d llmux -c 'CREATE EXTENSION IF NOT EXISTS vector;'"
# 预期输出: CREATE EXTENSION
```

### 4.3 Qdrant 测试
```bash
# 获取 Service IP
QDRANT_IP=$(kubectl get svc qdrant -n llmux-middleware -o jsonpath="{.spec.clusterIP}")

# 检查健康状态
curl http://$QDRANT_IP:6333/healthz
# 预期输出: ok
```

## 5. 连接信息

供 LLMux 应用程序配置使用（假设 LLMux 部署在同一集群）：

*   **Redis**: `redis.llmux-middleware.svc.cluster.local:6379`
*   **PostgreSQL**: `postgres.llmux-middleware.svc.cluster.local:5432`
*   **Qdrant**: `qdrant.llmux-middleware.svc.cluster.local:6333`

> **⚠️ 安全警告**: 默认密码为 `changeme_in_production`，请在 `k3s-middleware.yaml` 中修改后再部署。
