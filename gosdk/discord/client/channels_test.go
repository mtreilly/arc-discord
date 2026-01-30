package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

func TestChannelsGetChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/channels/123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.Channel{ID: "123", Name: "general"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)

	channel, err := client.Channels().GetChannel(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetChannel error: %v", err)
	}
	if channel.ID != "123" {
		t.Fatalf("expected channel ID 123, got %s", channel.ID)
	}
}

func TestChannelsModifyChannel(t *testing.T) {
	var receivedReason string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		receivedReason = r.Header.Get("X-Audit-Log-Reason")
		var payload types.ModifyChannelParams
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Topic != "Deployments" {
			t.Fatalf("expected topic Deployments, got %s", payload.Topic)
		}
		json.NewEncoder(w).Encode(types.Channel{ID: "123", Topic: payload.Topic})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	params := &types.ModifyChannelParams{
		Topic:          "Deployments",
		AuditLogReason: "Scheduled update",
	}

	channel, err := client.Channels().ModifyChannel(context.Background(), "123", params)
	if err != nil {
		t.Fatalf("ModifyChannel error: %v", err)
	}
	if channel.Topic != "Deployments" {
		t.Fatalf("expected updated topic, got %s", channel.Topic)
	}
	if receivedReason != url.QueryEscape("Scheduled update") {
		t.Fatalf("expected encoded audit log reason, got %s", receivedReason)
	}
}

func TestChannelsDeleteChannel(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Channels().DeleteChannel(context.Background(), "123"); err != nil {
		t.Fatalf("DeleteChannel error: %v", err)
	}
	if !called {
		t.Fatal("expected delete handler to run")
	}
}

func TestChannelsGetMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("expected limit=10, got %s", got)
		}
		if got := r.URL.Query().Get("before"); got != "555" {
			t.Fatalf("expected before=555, got %s", got)
		}
		json.NewEncoder(w).Encode([]*types.Message{
			{ID: "1", Content: "hello"},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	params := &GetChannelMessagesParams{
		Limit:  10,
		Before: "555",
	}

	msgs, err := client.Channels().GetChannelMessages(context.Background(), "123", params)
	if err != nil {
		t.Fatalf("GetChannelMessages error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "1" {
		t.Fatalf("unexpected messages %+v", msgs)
	}
}

func TestChannelsGetMessagesValidation(t *testing.T) {
	client := newTestClient(t, "http://example.com")
	params := &GetChannelMessagesParams{Limit: 500}
	if _, err := client.Channels().GetChannelMessages(context.Background(), "123", params); err == nil {
		t.Fatal("expected validation error")
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := New("token",
		WithBaseURL(baseURL),
		WithRateLimiter(&noopTracker{}),
		WithStrategy(ratelimit.NewReactiveStrategy()),
		WithHTTPClient(&http.Client{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}
