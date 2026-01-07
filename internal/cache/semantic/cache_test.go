package semantic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/blueberrycongee/llmux/internal/cache/semantic/vector"
)

// MockEmbedder is a mock implementation of embedding.Embedder
type MockEmbedder struct {
	mock.Mock
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	args := m.Called(ctx, texts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float64), args.Error(1)
}

func (m *MockEmbedder) Model() string {
	return "mock-embedding-model"
}

func (m *MockEmbedder) Dimension() int {
	return 1536
}

// MockVectorStore is a mock implementation of vector.Store
type MockVectorStore struct {
	mock.Mock
}

func (m *MockVectorStore) Search(ctx context.Context, vec []float64, opts vector.SearchOptions) ([]vector.SearchResult, error) {
	args := m.Called(ctx, vec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vector.SearchResult), args.Error(1)
}

func (m *MockVectorStore) Insert(ctx context.Context, entry vector.Entry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockVectorStore) InsertBatch(ctx context.Context, entries []vector.Entry) error {
	args := m.Called(ctx, entries)
	return args.Error(0)
}

func (m *MockVectorStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockVectorStore) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockVectorStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewCache(t *testing.T) {
	embedder := &MockEmbedder{}
	store := &MockVectorStore{}

	t.Run("should create cache with valid config", func(t *testing.T) {
		cfg := Config{
			SimilarityThreshold: 0.95,
			DefaultTTL:          time.Hour,
		}

		cache, err := New(embedder, store, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, 0.95, cache.SimilarityThreshold())
	})

	t.Run("should fail without embedder", func(t *testing.T) {
		cfg := DefaultConfig()
		cache, err := New(nil, store, cfg)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "embedder is required")
	})

	t.Run("should fail without vector store", func(t *testing.T) {
		cfg := DefaultConfig()
		cache, err := New(embedder, nil, cfg)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "vector store is required")
	})

	t.Run("should use default values for invalid config", func(t *testing.T) {
		cfg := Config{
			SimilarityThreshold: 0, // Invalid, should default to 0.95
			DefaultTTL:          0, // Invalid, should default to 1h
		}

		cache, err := New(embedder, store, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, 0.95, cache.SimilarityThreshold())
	})
}

func TestCacheGet(t *testing.T) {
	ctx := context.Background()
	testVector := []float64{0.1, 0.2, 0.3}

	t.Run("should return nil for empty prompt", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}
		cache, _ := New(embedder, store, DefaultConfig())

		result, err := cache.Get(ctx, "")
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should return cache hit when similarity exceeds threshold", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		embedder.On("Embed", ctx, "test prompt").Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.Anything).Return([]vector.SearchResult{
			{
				ID:       "test-id",
				Score:    0.98, // Above 0.95 threshold
				Distance: 0.02,
				Payload: vector.Payload{
					Prompt:   "test prompt",
					Response: "cached response",
					Model:    "gpt-4",
				},
			},
		}, nil)

		cfg := Config{SimilarityThreshold: 0.95, DefaultTTL: time.Hour}
		cache, _ := New(embedder, store, cfg)

		result, err := cache.Get(ctx, "test prompt")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "cached response", result.Response)
		assert.Equal(t, 0.98, result.Similarity)
		assert.Equal(t, "gpt-4", result.Model)

		embedder.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("should return nil when similarity below threshold", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		embedder.On("Embed", ctx, "test prompt").Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.Anything).Return([]vector.SearchResult{
			{
				ID:       "test-id",
				Score:    0.80, // Below 0.95 threshold
				Distance: 0.20,
				Payload: vector.Payload{
					Prompt:   "different prompt",
					Response: "cached response",
				},
			},
		}, nil)

		cfg := Config{SimilarityThreshold: 0.95, DefaultTTL: time.Hour}
		cache, _ := New(embedder, store, cfg)

		result, err := cache.Get(ctx, "test prompt")
		assert.NoError(t, err)
		assert.Nil(t, result)

		embedder.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("should return nil when no results found", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		embedder.On("Embed", ctx, "test prompt").Return(testVector, nil)
		store.On("Search", ctx, testVector, mock.Anything).Return([]vector.SearchResult{}, nil)

		cache, _ := New(embedder, store, DefaultConfig())

		result, err := cache.Get(ctx, "test prompt")
		assert.NoError(t, err)
		assert.Nil(t, result)

		embedder.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestCacheSet(t *testing.T) {
	ctx := context.Background()
	testVector := []float64{0.1, 0.2, 0.3}

	t.Run("should store response in cache", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		embedder.On("Embed", ctx, "test prompt").Return(testVector, nil)
		store.On("Insert", ctx, mock.MatchedBy(func(entry vector.Entry) bool {
			return entry.Payload.Prompt == "test prompt" &&
				entry.Payload.Response == "test response" &&
				entry.Payload.Model == "gpt-4"
		})).Return(nil)

		cache, _ := New(embedder, store, DefaultConfig())

		err := cache.Set(ctx, "test prompt", "test response", "gpt-4", time.Hour)
		assert.NoError(t, err)

		embedder.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("should skip empty prompt", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		cache, _ := New(embedder, store, DefaultConfig())

		err := cache.Set(ctx, "", "test response", "gpt-4", time.Hour)
		assert.NoError(t, err)

		// No calls should be made
		embedder.AssertNotCalled(t, "Embed")
		store.AssertNotCalled(t, "Insert")
	})

	t.Run("should skip empty response", func(t *testing.T) {
		embedder := &MockEmbedder{}
		store := &MockVectorStore{}

		cache, _ := New(embedder, store, DefaultConfig())

		err := cache.Set(ctx, "test prompt", "", "gpt-4", time.Hour)
		assert.NoError(t, err)

		// No calls should be made
		embedder.AssertNotCalled(t, "Embed")
		store.AssertNotCalled(t, "Insert")
	})
}

func TestCacheStats(t *testing.T) {
	embedder := &MockEmbedder{}
	store := &MockVectorStore{}
	cache, _ := New(embedder, store, DefaultConfig())

	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, int64(0), stats.Sets)
	assert.Equal(t, int64(0), stats.Errors)
	assert.Equal(t, float64(0), stats.HitRate)
}
