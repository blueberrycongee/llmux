// Package e2e contains end-to-end tests for LLMux.
package e2e

import (
	"os"
	"testing"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

var (
	// Global test fixtures
	mockLLM    *testutil.MockLLMServer
	testServer *testutil.TestServer
	testClient *testutil.TestClient
)

func TestMain(m *testing.M) {
	// Setup
	mockLLM = testutil.NewMockLLMServer()

	var err error
	testServer, err = testutil.NewTestServer(
		testutil.WithMockProvider(mockLLM.URL()),
		testutil.WithModels("gpt-4o-mock", "gpt-3.5-turbo-mock"),
	)
	if err != nil {
		panic("failed to create test server: " + err.Error())
	}

	if err := testServer.Start(); err != nil {
		panic("failed to start test server: " + err.Error())
	}

	testClient = testutil.NewTestClient(testServer.URL())

	// Run tests
	code := m.Run()

	// Teardown
	testServer.Stop()
	mockLLM.Close()

	os.Exit(code)
}

// resetMock resets the mock server state between tests.
func resetMock() {
	mockLLM.Reset()
}
