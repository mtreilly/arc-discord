package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestClient_SendToThread(t *testing.T) {
	threadID := "123456789"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery := r.URL.RawQuery

		// Verify thread_id is in query parameters
		if !strings.Contains(receivedQuery, "thread_id="+threadID) {
			t.Errorf("expected thread_id=%s in query, got: %s", threadID, receivedQuery)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	msg := &types.WebhookMessage{
		Content: "Test message in thread",
	}

	err = client.SendToThread(context.Background(), threadID, msg)
	if err != nil {
		t.Errorf("SendToThread() error = %v", err)
	}
}

func TestClient_SendToThread_Validation(t *testing.T) {
	client, err := NewClient("http://example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := []struct {
		name     string
		threadID string
		msg      *types.WebhookMessage
		wantErr  bool
	}{
		{
			name:     "empty thread ID",
			threadID: "",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			wantErr: true,
		},
		{
			name:     "valid thread ID",
			threadID: "123456",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a mock server that always succeeds
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client.webhookURL = server.URL

			err := client.SendToThread(context.Background(), tt.threadID, tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendToThread() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_CreateThread(t *testing.T) {
	threadName := "New Discussion Thread"
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the JSON body
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify thread_name is in the body
		if receivedBody["thread_name"] != threadName {
			t.Errorf("expected thread_name=%s, got: %v", threadName, receivedBody["thread_name"])
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	msg := &types.WebhookMessage{
		Content: "First message in new thread",
	}

	err = client.CreateThread(context.Background(), threadName, msg)
	if err != nil {
		t.Errorf("CreateThread() error = %v", err)
	}

	// Verify the message has thread_name set
	if receivedBody["thread_name"] != threadName {
		t.Errorf("thread_name not set correctly in request")
	}
}

func TestClient_CreateThread_Validation(t *testing.T) {
	client, err := NewClient("http://example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := []struct {
		name       string
		threadName string
		msg        *types.WebhookMessage
		wantErr    bool
	}{
		{
			name:       "empty thread name",
			threadName: "",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			wantErr: true,
		},
		{
			name:       "valid thread name",
			threadName: "Discussion",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			wantErr: false,
		},
		{
			name:       "thread name too long",
			threadName: strings.Repeat("a", 101),
			msg: &types.WebhookMessage{
				Content: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a mock server that always succeeds
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client.webhookURL = server.URL

			err := client.CreateThread(context.Background(), tt.threadName, tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateThread() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebhookMessage_ThreadValidation(t *testing.T) {
	tests := []struct {
		name    string
		msg     *types.WebhookMessage
		wantErr bool
	}{
		{
			name: "valid message with thread_name",
			msg: &types.WebhookMessage{
				Content:    "test",
				ThreadName: "Discussion",
			},
			wantErr: false,
		},
		{
			name: "thread_name too long",
			msg: &types.WebhookMessage{
				Content:    "test",
				ThreadName: strings.Repeat("a", 101),
			},
			wantErr: true,
		},
		{
			name: "valid message with thread_id",
			msg: &types.WebhookMessage{
				Content:  "test",
				ThreadID: "123456789",
			},
			wantErr: false,
		},
		{
			name: "both thread_id and thread_name set",
			msg: &types.WebhookMessage{
				Content:    "test",
				ThreadID:   "123",
				ThreadName: "should-error",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_SendWithFiles_Thread(t *testing.T) {
	threadID := "987654321"
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery

		// Verify thread_id is in query parameters
		if !strings.Contains(receivedQuery, "thread_id="+threadID) {
			t.Errorf("expected thread_id=%s in query, got: %s", threadID, receivedQuery)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	msg := &types.WebhookMessage{
		Content:  "File upload in thread",
		ThreadID: threadID,
	}

	files := []FileAttachment{
		{
			Name:        "test.txt",
			ContentType: "text/plain",
			Reader:      strings.NewReader("test content"),
			Size:        12,
		},
	}

	err = client.SendWithFiles(context.Background(), msg, files)
	if err != nil {
		t.Errorf("SendWithFiles() error = %v", err)
	}
}

func TestBuildURLWithThreadID(t *testing.T) {
	client, _ := NewClient("http://example.com/webhook")

	tests := []struct {
		name     string
		baseURL  string
		threadID string
		want     string
	}{
		{
			name:     "no thread ID",
			baseURL:  "http://example.com/webhook",
			threadID: "",
			want:     "http://example.com/webhook",
		},
		{
			name:     "with thread ID",
			baseURL:  "http://example.com/webhook",
			threadID: "123456",
			want:     "http://example.com/webhook?thread_id=123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.buildURLWithThreadID(tt.baseURL, tt.threadID)
			if got != tt.want {
				t.Errorf("buildURLWithThreadID() = %v, want %v", got, tt.want)
			}
		})
	}
}
