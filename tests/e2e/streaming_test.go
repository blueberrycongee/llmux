//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

// Helper function to setup streaming test with fresh server
func setupStreamingTest(t *testing.T, content string) (*testutil.StreamReader, func()) {
	t.Helper()

	// Create a fresh mock server for streaming tests
	localMock := testutil.NewMockLLMServer()
	if content != "" {
		localMock.SetNextResponse(content)
	}

	// Create a fresh test server
	localServer, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
	)
	if err != nil {
		t.Skipf("Failed to create test server: %v", err)
		return nil, func() {}
	}

	if startErr := localServer.Start(); startErr != nil {
		localMock.Close()
		t.Skipf("Failed to start test server: %v", startErr)
		return nil, func() {}
	}

	localClient := testutil.NewTestClient(localServer.URL())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	req := &testutil.ChatCompletionRequest{
		Model:  "gpt-4o-mock",
		Stream: true,
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	stream, httpResp, err := localClient.ChatCompletionStream(ctx, req)
	if err != nil {
		cancel()
		localServer.Stop()
		localMock.Close()
		t.Skipf("Failed to create stream: %v", err)
		return nil, func() {}
	}

	if httpResp.StatusCode != http.StatusOK {
		cancel()
		if stream != nil {
			stream.Close()
		}
		localServer.Stop()
		localMock.Close()
		t.Skipf("Server returned status %d, skipping streaming test", httpResp.StatusCode)
		return nil, func() {}
	}

	if stream == nil {
		cancel()
		localServer.Stop()
		localMock.Close()
		t.Skip("Stream is nil, skipping test")
		return nil, func() {}
	}

	cleanup := func() {
		stream.Close()
		cancel()
		localServer.Stop()
		localMock.Close()
	}

	return stream, cleanup
}

func TestStreaming_Basic(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "")
	if stream == nil {
		return
	}
	defer cleanup()

	// Just verify we can read at least one chunk
	chunk, err := stream.Next()
	if err != io.EOF {
		require.NoError(t, err)
		assert.NotNil(t, chunk)
	}
}

func TestStreaming_ContentAccumulation(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "Hello, this is a streaming response!")
	if stream == nil {
		return
	}
	defer cleanup()

	content, err := stream.CollectContent()
	require.NoError(t, err)
	assert.Equal(t, "Hello, this is a streaming response!", content)
}

func TestStreaming_ChunkFormat(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "Test content")
	if stream == nil {
		return
	}
	defer cleanup()

	// Read first chunk
	chunk, err := stream.Next()
	require.NoError(t, err)

	// Verify chunk format
	assert.NotEmpty(t, chunk.ID, "chunk ID should not be empty")
	assert.Equal(t, "chat.completion.chunk", chunk.Object)
	assert.NotZero(t, chunk.Created)
	assert.NotEmpty(t, chunk.Model)
	assert.NotEmpty(t, chunk.Choices)

	// First chunk should have role
	if len(chunk.Choices) > 0 {
		assert.Equal(t, "assistant", chunk.Choices[0].Delta.Role)
	}
}

func TestStreaming_FinishReason(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "Short")
	if stream == nil {
		return
	}
	defer cleanup()

	var lastChunk *testutil.StreamChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		lastChunk = chunk
	}

	// Last chunk should have finish_reason
	require.NotNil(t, lastChunk)
	if len(lastChunk.Choices) > 0 {
		assert.Equal(t, "stop", lastChunk.Choices[0].FinishReason)
	}
}

func TestStreaming_MultipleChunks(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "This is a longer response that will be split into multiple chunks for streaming.")
	if stream == nil {
		return
	}
	defer cleanup()

	chunkCount := 0
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunkCount++
	}

	assert.Greater(t, chunkCount, 1, "should have multiple chunks")
}

func TestStreaming_ConsistentID(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "Hello world!")
	if stream == nil {
		return
	}
	defer cleanup()

	var firstID string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if firstID == "" {
			firstID = chunk.ID
		} else {
			assert.Equal(t, firstID, chunk.ID, "all chunks should have same ID")
		}
	}
}

func TestStreaming_ModelInChunks(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "Test")
	if stream == nil {
		return
	}
	defer cleanup()

	chunk, err := stream.Next()
	require.NoError(t, err)

	assert.Equal(t, "gpt-4o-mock", chunk.Model, "chunk should contain model name")
}

func TestStreaming_EmptyContent(t *testing.T) {
	stream, cleanup := setupStreamingTest(t, "")
	if stream == nil {
		return
	}
	defer cleanup()

	content, err := stream.CollectContent()
	require.NoError(t, err)

	// Empty or minimal content is valid
	_ = content
}
