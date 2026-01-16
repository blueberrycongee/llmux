# LLMux 缓存模块

此目录包含 LLMux 的缓存逻辑，包括精确匹配缓存和语义缓存。

## 缓存类型

1. **内存缓存 (Memory Cache)**: 本地内存缓存，用于快速查找。
2. **Redis 缓存 (Redis Cache)**: 分布式缓存，用于多实例部署。
3. **双级缓存 (Dual Cache)**: 结合内存和 Redis 的缓存，采用本地优先策略。
4. **语义缓存 (Semantic Cache)**: 基于向量嵌入的相似度缓存。

## 语义缓存流水线 (Pipeline)

语义缓存允许在 Prompt 不完全相同但语义相似时命中。它遵循多阶段流水线：

1. **Prompt 处理**: 清洗并准备输入的 Prompt。
2. **向量化 (Embedding Generation)**: 使用配置的向量模型（如 OpenAI 的 `text-embedding-3-small`）将 Prompt 转换为高维向量。
3. **向量检索**:
    - 使用该向量查询向量数据库（如 Qdrant）。
    - 若未开启重排序 (Re-ranking)，检索 Top-1 结果。
    - 若开启重排序，检索 Top-N（默认为 5）个候选者。
4. **初次过滤**: 根据 `SimilarityThreshold` (余弦相似度) 过滤候选者。
5. **重排序 (Re-ranking - 可选)**:
    - 若开启，则使用二次指标（如 Jaccard 字符串相似度）进一步评估候选者。
    - 确保在语义相似的结果中，挑选出在文本层面上与原始 Prompt 最匹配的一个，减少“幻觉”式缓存命中。
    - 结果必须通过 `RerankingThreshold` 阈值才被视为命中。
6. **结果返回**: 返回匹配度最高的候选者的缓存响应。

## 配置

语义缓存可以通过 `semantic.Config` 进行配置：

| 参数 | 描述 | 默认值 |
|-----------|-------------|---------|
| `SimilarityThreshold` | 向量搜索的最小余弦相似度 | 0.95 |
| `EnableReranking` | 开启二次字符串相似度校验 | `false` |
| `RerankingThreshold` | 二次相似度的最小分值 | 0.8 |
| `EmbeddingModel` | 用于向量化的模型 | `text-embedding-ada-002` |
| `VectorStore` | 向量存储后端 (如 `qdrant`) | `qdrant` |
