package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUserInfoClient_GetUserInfo(t *testing.T) {
	// Create mock UserInfo endpoint
	expectedInfo := UserInfoResponse{
		Subject:           "user-123",
		Name:              "John Doe",
		Email:             "john@example.com",
		EmailVerified:     true,
		PreferredUsername: "johndoe",
		Groups:            []string{"admins", "developers"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedInfo)
	}))
	defer server.Close()

	client := NewUserInfoClient(server.URL, 5*time.Minute)

	// Test successful fetch
	info, err := client.GetUserInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetUserInfo failed: %v", err)
	}

	if info.Subject != expectedInfo.Subject {
		t.Errorf("Subject mismatch: got %s, want %s", info.Subject, expectedInfo.Subject)
	}

	if info.Email != expectedInfo.Email {
		t.Errorf("Email mismatch: got %s, want %s", info.Email, expectedInfo.Email)
	}

	if len(info.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(info.Groups))
	}
}

func TestUserInfoClient_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfoResponse{
			Subject: "user-456",
			Email:   "test@example.com",
		})
	}))
	defer server.Close()

	client := NewUserInfoClient(server.URL, 5*time.Minute)

	// First call - should hit the server
	_, err := client.GetUserInfo(context.Background(), "cache-token")
	if err != nil {
		t.Fatalf("First GetUserInfo failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}

	// Second call - should use cache
	_, err = client.GetUserInfo(context.Background(), "cache-token")
	if err != nil {
		t.Fatalf("Second GetUserInfo failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected still 1 server call (cached), got %d", callCount)
	}

	// Different token - should hit server again
	_, err = client.GetUserInfo(context.Background(), "different-token")
	if err != nil {
		t.Fatalf("Third GetUserInfo failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 server calls, got %d", callCount)
	}
}

func TestUserInfoClient_CacheExpiration(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfoResponse{
			Subject: "user-789",
		})
	}))
	defer server.Close()

	// Use very short TTL for testing
	client := NewUserInfoClient(server.URL, 50*time.Millisecond)

	// First call
	_, _ = client.GetUserInfo(context.Background(), "expire-token")
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Should call server again
	_, _ = client.GetUserInfo(context.Background(), "expire-token")
	if callCount != 2 {
		t.Errorf("Expected 2 calls after expiration, got %d", callCount)
	}
}

func TestUserInfoClient_ClearCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfoResponse{Subject: "test"})
	}))
	defer server.Close()

	client := NewUserInfoClient(server.URL, 5*time.Minute)

	// Populate cache
	_, _ = client.GetUserInfo(context.Background(), "token1")
	_, _ = client.GetUserInfo(context.Background(), "token2")

	if client.CacheSize() != 2 {
		t.Errorf("Expected cache size 2, got %d", client.CacheSize())
	}

	// Clear cache
	client.ClearCache()

	if client.CacheSize() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", client.CacheSize())
	}
}

func TestUserInfoClient_ErrorHandling(t *testing.T) {
	// Test 401 Unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewUserInfoClient(server.URL, 5*time.Minute)

	_, err := client.GetUserInfo(context.Background(), "bad-token")
	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

func TestUserInfoResponse_GetClaims(t *testing.T) {
	info := &UserInfoResponse{
		Subject: "user-abc",
		Email:   "test@example.com",
		RawClaims: map[string]any{
			"sub":           "user-abc",
			"email":         "test@example.com",
			"custom_string": "value",
			"custom_array":  []any{"a", "b", "c"},
		},
	}

	// Test GetStringClaim
	if val := info.GetStringClaim("custom_string"); val != "value" {
		t.Errorf("Expected 'value', got '%s'", val)
	}

	if val := info.GetStringClaim("nonexistent"); val != "" {
		t.Errorf("Expected empty string for nonexistent claim, got '%s'", val)
	}

	// Test GetStringSliceClaim
	arr := info.GetStringSliceClaim("custom_array")
	if len(arr) != 3 {
		t.Errorf("Expected 3 items, got %d", len(arr))
	}
}

func TestUserInfoClient_ClearExpiredCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfoResponse{Subject: "test"})
	}))
	defer server.Close()

	client := NewUserInfoClient(server.URL, 50*time.Millisecond)

	// Populate cache
	_, _ = client.GetUserInfo(context.Background(), "token1")
	_, _ = client.GetUserInfo(context.Background(), "token2")

	if client.CacheSize() != 2 {
		t.Errorf("Expected cache size 2, got %d", client.CacheSize())
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Clear expired entries
	cleared := client.ClearExpiredCache()
	if cleared != 2 {
		t.Errorf("Expected 2 cleared entries, got %d", cleared)
	}

	if client.CacheSize() != 0 {
		t.Errorf("Expected cache size 0, got %d", client.CacheSize())
	}
}
