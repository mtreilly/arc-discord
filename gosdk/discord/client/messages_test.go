package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestMessageServiceCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/channels/123/messages" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload types.MessageCreateParams
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Content != "hello" {
			t.Fatalf("expected content hello, got %s", payload.Content)
		}
		json.NewEncoder(w).Encode(types.Message{ID: "42", Content: payload.Content})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	msg, err := client.Messages().CreateMessage(context.Background(), "123", &types.MessageCreateParams{
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("CreateMessage error: %v", err)
	}
	if msg.ID != "42" {
		t.Fatalf("expected message ID 42, got %s", msg.ID)
	}
}

func TestMessageServiceEdit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH")
		}
		json.NewEncoder(w).Encode(types.Message{ID: "55", Content: "updated"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	msg, err := client.Messages().EditMessage(context.Background(), "123", "55", &types.MessageEditParams{
		Content: "updated",
	})
	if err != nil {
		t.Fatalf("EditMessage error: %v", err)
	}
	if msg.Content != "updated" {
		t.Fatalf("expected updated content, got %s", msg.Content)
	}
}

func TestMessageServiceDelete(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Messages().DeleteMessage(context.Background(), "123", "55"); err != nil {
		t.Fatalf("DeleteMessage error: %v", err)
	}
	if !called {
		t.Fatal("expected delete handler to run")
	}
}

func TestMessageServiceBulkDelete(t *testing.T) {
	var payload struct {
		Messages []string `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/channels/123/messages/bulk-delete" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(payload.Messages))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.Messages().BulkDeleteMessages(context.Background(), "123", []string{"1", "2"})
	if err != nil {
		t.Fatalf("BulkDeleteMessages error: %v", err)
	}
}

func TestMessageServiceGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(types.Message{ID: "77", Content: "ping"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	msg, err := client.Messages().GetMessage(context.Background(), "123", "77")
	if err != nil {
		t.Fatalf("GetMessage error: %v", err)
	}
	if msg.ID != "77" {
		t.Fatalf("expected message ID 77")
	}
}

func TestMessageServiceValidation(t *testing.T) {
	client := newTestClient(t, "http://example.com")

	if _, err := client.Messages().CreateMessage(context.Background(), "", &types.MessageCreateParams{}); err == nil {
		t.Fatal("expected error for empty channel ID")
	}
	if err := client.Messages().DeleteMessage(context.Background(), "123", ""); err == nil {
		t.Fatal("expected error for empty message ID")
	}
	if err := client.Messages().BulkDeleteMessages(context.Background(), "123", nil); err == nil {
		t.Fatal("expected error for nil message IDs")
	}
	if err := client.Messages().BulkDeleteMessages(context.Background(), "123", make([]string, 101)); err == nil {
		t.Fatal("expected error for >100 IDs")
	}
	if err := client.Messages().CreateReaction(context.Background(), "", "1", ":smile:"); err == nil {
		t.Fatal("expected error for empty channel ID in reaction")
	}
}

func TestMessageServiceCreateReaction(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.RequestURI
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Messages().CreateReaction(context.Background(), "123", "456", ":smile:"); err != nil {
		t.Fatalf("CreateReaction error: %v", err)
	}
	if path != "/channels/123/messages/456/reactions/%3Asmile%3A/@me" {
		t.Fatalf("unexpected path %s", path)
	}
}

func TestMessageServiceDeleteAllReactions(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.RequestURI != "/channels/123/messages/456/reactions/%F0%9F%98%80" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Messages().DeleteAllReactions(context.Background(), "123", "456", "😀"); err != nil {
		t.Fatalf("DeleteAllReactions error: %v", err)
	}
	if !called {
		t.Fatal("expected delete all reactions to be called")
	}
}

func TestMessageServiceGetReactions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Fatalf("expected limit=25, got %s", got)
		}
		json.NewEncoder(w).Encode([]*types.User{{ID: "u1"}})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	users, err := client.Messages().GetReactions(context.Background(), "123", "456", "🔥", &GetReactionsParams{Limit: 25})
	if err != nil {
		t.Fatalf("GetReactions error: %v", err)
	}
	if len(users) != 1 || users[0].ID != "u1" {
		t.Fatalf("unexpected users %+v", users)
	}
}
