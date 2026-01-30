package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		wantErr    bool
	}{
		{"valid URL", "https://discord.com/api/webhooks/123/abc", false},
		{"empty URL", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.webhookURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_SendSimple(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		var msg types.WebhookMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if msg.Content != "test message" {
			t.Errorf("Expected content 'test message', got '%s'", msg.Content)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	err = client.SendSimple(ctx, "test message")
	if err != nil {
		t.Errorf("SendSimple() error = %v", err)
	}
}

func TestClient_SendWithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(3), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	err = client.SendSimple(ctx, "test message")
	if err != nil {
		t.Errorf("SendSimple() error = %v, expected success after retries", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "You are being rate limited.",
			"retry_after": 0.5,
			"global":      false,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(2))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	start := time.Now()
	err = client.SendSimple(ctx, "test message")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error for rate limit, got nil")
	}

	// Should have retried with backoff
	if elapsed < 500*time.Millisecond {
		t.Errorf("Expected at least 500ms delay for rate limit, got %v", elapsed)
	}
}
