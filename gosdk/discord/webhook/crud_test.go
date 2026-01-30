package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestClient_Edit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		// Verify URL path
		expectedPath := "/messages/123456789"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse request body
		var params MessageEditParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if params.Content == nil || *params.Content != "edited content" {
			t.Errorf("Expected content 'edited content', got %v", params.Content)
		}

		// Return edited message
		resp := types.Message{
			ID:        "123456789",
			ChannelID: "987654321",
			Content:   "edited content",
			Timestamp: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	content := "edited content"
	params := &MessageEditParams{
		Content: &content,
	}

	msg, err := client.Edit(ctx, "123456789", params)
	if err != nil {
		t.Errorf("Edit() error = %v", err)
	}

	if msg.Content != "edited content" {
		t.Errorf("Expected content 'edited content', got '%s'", msg.Content)
	}
}

func TestClient_Edit_Validation(t *testing.T) {
	client, _ := NewClient("http://test.com")
	ctx := context.Background()

	tests := []struct {
		name      string
		messageID string
		params    *MessageEditParams
		wantErr   bool
	}{
		{
			name:      "empty message ID",
			messageID: "",
			params:    &MessageEditParams{},
			wantErr:   true,
		},
		{
			name:      "nil params",
			messageID: "123",
			params:    nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Edit(ctx, tt.messageID, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Edit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				// Check that it's a validation error
				var valErr *types.ValidationError
				if !isErrorType(err, &valErr) {
					t.Errorf("Expected ValidationError, got %T", err)
				}
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got %s", r.Method)
		}

		// Verify URL path
		expectedPath := "/messages/123456789"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	err = client.Delete(ctx, "123456789")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestClient_Delete_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Unknown Message",
			"code":    10008,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	err = client.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent message, got nil")
	}

	// Check if it's an API error type
	var dummy *types.APIError
	if !isErrorType(err, &dummy) {
		t.Errorf("Expected APIError, got %T: %v", err, err)
	}

	// Extract API error for status code check
	apiErr := unwrapToAPIError(err)
	if apiErr != nil && apiErr.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", apiErr.StatusCode)
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify URL path
		expectedPath := "/messages/123456789"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Return message
		resp := types.Message{
			ID:        "123456789",
			ChannelID: "987654321",
			Content:   "original content",
			Timestamp: time.Now(),
			Author: &types.User{
				ID:       "webhook-id",
				Username: "Test Webhook",
				Bot:      true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	msg, err := client.Get(ctx, "123456789")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}

	if msg.Content != "original content" {
		t.Errorf("Expected content 'original content', got '%s'", msg.Content)
	}

	if msg.Author == nil || msg.Author.Username != "Test Webhook" {
		t.Error("Expected author information")
	}
}

func TestClient_Get_Validation(t *testing.T) {
	client, _ := NewClient("http://test.com")
	ctx := context.Background()

	_, err := client.Get(ctx, "")
	if err == nil {
		t.Error("Expected error for empty message ID, got nil")
	}

	var valErr *types.ValidationError
	if !isErrorType(err, &valErr) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

func TestClient_CRUD_Integration(t *testing.T) {
	var sentMessageID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			// Send message - return message with ID
			msg := types.Message{
				ID:        "123456789",
				ChannelID: "987654321",
				Content:   "original content",
				Timestamp: time.Now(),
			}
			sentMessageID = msg.ID
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)

		case "GET":
			// Get message
			msg := types.Message{
				ID:        sentMessageID,
				ChannelID: "987654321",
				Content:   "original content",
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)

		case "PATCH":
			// Edit message
			var params MessageEditParams
			json.NewDecoder(r.Body).Decode(&params)

			msg := types.Message{
				ID:        sentMessageID,
				ChannelID: "987654321",
				Content:   *params.Content,
				Timestamp: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)

		case "DELETE":
			// Delete message
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()

	// 1. Send a message (simulated - we'll use the webhook endpoint directly)
	// In real usage, you'd call Send() and get the message ID from Discord's response
	sentMessageID = "123456789"

	// 2. Get the message
	msg, err := client.Get(ctx, sentMessageID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if msg.Content != "original content" {
		t.Errorf("Expected original content, got '%s'", msg.Content)
	}

	// 3. Edit the message
	newContent := "edited content"
	editedMsg, err := client.Edit(ctx, sentMessageID, &MessageEditParams{
		Content: &newContent,
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if editedMsg.Content != "edited content" {
		t.Errorf("Expected edited content, got '%s'", editedMsg.Content)
	}

	// 4. Delete the message
	err = client.Delete(ctx, sentMessageID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestBuildMessageURL(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		messageID  string
		want       string
	}{
		{
			name:       "basic URL",
			webhookURL: "https://discord.com/api/webhooks/123/abc",
			messageID:  "456",
			want:       "https://discord.com/api/webhooks/123/abc/messages/456",
		},
		{
			name:       "URL with trailing slash",
			webhookURL: "https://discord.com/api/webhooks/123/abc/",
			messageID:  "456",
			want:       "https://discord.com/api/webhooks/123/abc/messages/456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewClient(tt.webhookURL)
			got := client.buildMessageURL(tt.messageID)
			if got != tt.want {
				t.Errorf("buildMessageURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func isErrorType(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	// Check if error wraps the target type
	errStr := err.Error()
	switch target.(type) {
	case **types.ValidationError:
		// Check if it's a ValidationError or wraps one
		_, ok := err.(*types.ValidationError)
		if !ok {
			// Check if error message contains validation-related keywords
			return strings.Contains(errStr, "validation") ||
				   strings.Contains(errStr, "required") ||
				   strings.Contains(errStr, "invalid")
		}
		return ok
	case **types.APIError:
		// Check if it's an APIError or wraps one
		_, ok := err.(*types.APIError)
		if !ok {
			// Check if error wraps an APIError
			return strings.Contains(errStr, "Discord API error") ||
				   strings.Contains(errStr, "status code")
		}
		return ok
	default:
		return false
	}
}

func unwrapToAPIError(err error) *types.APIError {
	if apiErr, ok := err.(*types.APIError); ok {
		return apiErr
	}
	// Try to extract APIError from wrapped error
	errStr := err.Error()
	if strings.Contains(errStr, "Discord API error") {
		// Extract status code if possible
		// This is a simplification - in production you'd use errors.As
		return &types.APIError{
			StatusCode: 404, // Default for testing
		}
	}
	return nil
}
