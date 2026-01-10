# Advanced Agent Memory System

The `internal/memory` package provides a sophisticated, multi-layered memory system for LLMux Agents, designed to mimic human-like recall and learning. It goes beyond simple message history, enabling agents to store, manage, and retrieve structured knowledge over long periods.

Inspired by [Mem0](https://github.com/mem0ai/mem0), this system implements a tiered architecture with smart ingestion and hybrid retrieval capabilities.

## 🧠 Core Architecture

The system is organized into three primary layers, orchestrated by the `MemoryManager`:

1.  **Short-term Memory (Session)**
    *   **Purpose**: Maintains the immediate conversational context.
    *   **Mechanism**: Stores raw `ChatMessage` history.
    *   **Storage**: In-Memory or Redis (via `SessionStore` interface).

2.  **Long-term Memory (Vector / RAG)**
    *   **Purpose**: Stores factual knowledge, user preferences, and past experiences.
    *   **Mechanism**: Uses Vector Embeddings and Hybrid Similarity Search.
    *   **Storage**: Vector Database (via `VectorStore` interface).

3.  **Entity Memory (Structured)**
    *   **Purpose**: Tracks specific attributes of entities (Users, Agents).
    *   **Mechanism**: Structured metadata attached to memory entries.

## ✨ Key Features

### 1. Smart Ingestion (LLM-Driven)
Instead of just storing raw text, the system uses an LLM to **extract** meaningful facts from user input.
*   **Extraction**: Identifies core facts, entities, and categories (e.g., "hobbies", "work").
*   **Action Determination**: The LLM decides whether to `ADD` a new memory, `UPDATE` an existing one, or `DELETE` an obsolete one.

### 2. Dynamic Conflict Resolution
The system automatically handles contradictions and updates:
*   **Deduplication**: Uses cosine similarity (threshold `>0.9`) to detect if a "new" fact is just a rephrase of an existing one.
*   **Resolution**:
    *   **ADD**: Adds new information if no conflict exists.
    *   **UPDATE**: Replaces old information with new details (e.g., "I moved to Berlin" replaces "I live in Paris").
    *   **DELETE**: Removes specific memories upon request (e.g., "Forget my phone number").

### 3. Hybrid Retrieval (Semantic + Recency)
Retrieval isn't just about matching keywords. We use a hybrid scoring formula to surface the most relevant *and* current information:

$$ Score = (SemanticScore \times 0.8) + (RecencyScore \times 0.2) $$

*   **Semantic Score**: Cosine similarity between query and memory vectors.
*   **Recency Score**: Exponential decay function `exp(-0.01 * hours_passed)`, prioritizing newer memories.

## 🚀 Usage

### Initialization

```go
import (
    "github.com/blueberrycongee/llmux/internal/memory"
    "github.com/blueberrycongee/llmux/internal/memory/inmem"
)

// 1. Initialize Components
sessionStore := inmem.NewInMemorySessionStore()
vectorStore := inmem.NewMemoryVectorStore() // Or Qdrant/Milvus
llmClient := inmem.NewRealLLMClientSimulator() // Or OpenAI Client
embedder := &inmem.SimpleEmbedder{} // Or OpenAI Embedder

// 2. Create Manager
memManager := memory.NewMemoryManager(sessionStore, vectorStore, embedder, llmClient)
```

### Ingesting Memory (Smart Mode)

Pass raw user input to `IngestMemory`. The system will analyze it and update the state.

```go
ctx := context.Background()
userID := "user_123"
userInput := "I just moved to Tokyo and I love sushi now."

// Extracts: "User lives in Tokyo", "User likes sushi"
// Actions: ADD / UPDATE
err := memManager.IngestMemory(ctx, userID, userInput)
```

### Retrieving Context

When generating a response, fetch relevant context:

```go
query := "What should I eat for dinner?"
contextStr, err := memManager.RetrieveRelevantContext(ctx, query, userID)

// Result: "User likes sushi. User lives in Tokyo."
```

## 🛠 Configuration

The system is highly modular. You can implement your own backends by satisfying these interfaces:

*   **`VectorStore`**: For storing embeddings (`Search`, `Add`, `Delete`).
*   **`LLMClient`**: For the extraction/resolution step (`Call`).
*   **`Embedder`**: For generating vector embeddings (`Embed`).
*   **`SessionStore`**: For raw chat history.

## 🧪 Testing

The module includes a comprehensive test suite using a **Deterministic Simulator** instead of mocks, ensuring logic flows are tested with "real" behavior without external API costs.

```bash
go test ./internal/memory/tests/...
```
