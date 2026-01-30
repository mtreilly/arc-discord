package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestClient_NetworkErrors(t *testing.T) {
	// Server that never responds (will timeout)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		// Don't write response - simulates network issue
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithTimeout(50*time.Millisecond), WithMaxRetries(1))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	msg := &types.WebhookMessage{Content: "test"}
	err = client.Send(context.Background(), msg)
	if err == nil {
		t.Error("Send() expected error for network timeout, got nil")
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	msg := &types.WebhookMessage{Content: "test"}
	err = client.Send(ctx, msg)
	if err == nil {
		t.Error("Send() expected context timeout error, got nil")
	}
}

func TestClient_ServerErrors(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		// Return 500 Internal Server Error (should retry)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Internal server error"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(2))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	msg := &types.WebhookMessage{Content: "test"}
	err = client.Send(context.Background(), msg)
	if err == nil {
		t.Error("Send() expected error for 500 response, got nil")
	}

	// Should have retried 3 times (initial + 2 retries)
	if attempt != 3 {
		t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempt)
	}
}

func TestClient_Delete_ServerError(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Server error"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(2))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Delete(context.Background(), "123456")
	if err == nil {
		t.Error("Delete() expected error for 500 response, got nil")
	}

	if attempt != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempt)
	}
}

func TestClient_Edit_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"Service unavailable"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(1))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	content := "updated content"
	params := &MessageEditParams{
		Content: &content,
	}

	_, err = client.Edit(context.Background(), "123456", params)
	if err == nil {
		t.Error("Edit() expected error for 503 response, got nil")
	}
}

func TestClient_Get_JSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Invalid JSON
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Get(context.Background(), "123456")
	if err == nil {
		t.Error("Get() expected JSON decode error, got nil")
	}
}

func TestClient_SendWithFiles_NetworkError(t *testing.T) {
	client, err := NewClient("http://localhost:1", WithMaxRetries(1), WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	msg := &types.WebhookMessage{Content: "test"}
	files := []FileAttachment{
		{
			Name:        "test.txt",
			ContentType: "text/plain",
			Reader:      strings.NewReader("test"),
			Size:        4,
		},
	}

	err = client.SendWithFiles(context.Background(), msg, files)
	if err == nil {
		t.Error("SendWithFiles() expected network error, got nil")
	}
}

func TestClient_Delete_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = client.Delete(ctx, "123456")
	if err == nil {
		t.Error("Delete() expected context timeout error, got nil")
	}
}

func TestClient_Edit_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	content := "test"
	params := &MessageEditParams{Content: &content}

	_, err = client.Edit(ctx, "123456", params)
	if err == nil {
		t.Error("Edit() expected context timeout error, got nil")
	}
}

func TestClient_ClientErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantRetry  bool
	}{
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"Bad request"}`,
			wantRetry:  false, // Client errors don't retry
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"message":"Unauthorized"}`,
			wantRetry:  false,
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"message":"Forbidden"}`,
			wantRetry:  false,
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Not found"}`,
			wantRetry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt++
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, WithMaxRetries(2))
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			msg := &types.WebhookMessage{Content: "test"}
			err = client.Send(context.Background(), msg)
			if err == nil {
				t.Errorf("Send() expected error for %d response, got nil", tt.statusCode)
			}

			// Client errors should not retry
			if !tt.wantRetry && attempt != 1 {
				t.Errorf("Expected 1 attempt (no retry), got %d", attempt)
			}
		})
	}
}
