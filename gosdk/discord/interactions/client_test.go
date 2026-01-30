package interactions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

func TestInteractionClientCreateInteractionResponse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var called bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/interactions/abc/token/callback" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			var payload types.InteractionResponse
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Type != types.InteractionResponseChannelMessageWithSource {
				t.Fatalf("unexpected response type %d", payload.Type)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		ic := NewInteractionClient(newInteractionTestClient(t, server.URL))
		resp := &types.InteractionResponse{
			Type: types.InteractionResponseChannelMessageWithSource,
			Data: &types.InteractionApplicationCommandCallbackData{
				Content: "hello",
			},
		}
		if err := ic.CreateInteractionResponse(context.Background(), "abc", "token", resp); err != nil {
			t.Fatalf("CreateInteractionResponse error: %v", err)
		}
		if !called {
			t.Fatal("expected handler invocation")
		}
	})

	t.Run("validation", func(t *testing.T) {
		ic := NewInteractionClient(newInteractionTestClient(t, "http://example.com"))
		err := ic.CreateInteractionResponse(context.Background(), "", "token", &types.InteractionResponse{})
		if err == nil {
			t.Fatal("expected validation error for missing interaction ID")
		}
	})
}

func TestInteractionClientOriginalResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/webhooks/app/token/messages/@original":
			_ = json.NewEncoder(w).Encode(types.Message{ID: "123"})
		case r.Method == http.MethodPatch && r.URL.Path == "/webhooks/app/token/messages/@original":
			var payload types.MessageEditParams
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Content != "updated" {
				t.Fatalf("expected updated content, got %s", payload.Content)
			}
			_ = json.NewEncoder(w).Encode(types.Message{ID: "123", Content: payload.Content})
		case r.Method == http.MethodDelete && r.URL.Path == "/webhooks/app/token/messages/@original":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	ic := NewInteractionClient(newInteractionTestClient(t, server.URL))

	msg, err := ic.GetOriginalInteractionResponse(context.Background(), "app", "token")
	if err != nil {
		t.Fatalf("GetOriginalInteractionResponse error: %v", err)
	}
	if msg.ID != "123" {
		t.Fatalf("expected message ID 123, got %s", msg.ID)
	}

	edited, err := ic.EditOriginalInteractionResponse(context.Background(), "app", "token", &types.MessageEditParams{Content: "updated"})
	if err != nil {
		t.Fatalf("EditOriginalInteractionResponse error: %v", err)
	}
	if edited.Content != "updated" {
		t.Fatalf("expected updated content, got %s", edited.Content)
	}

	if err := ic.DeleteOriginalInteractionResponse(context.Background(), "app", "token"); err != nil {
		t.Fatalf("DeleteOriginalInteractionResponse error: %v", err)
	}
}

func TestInteractionClientFollowupMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/webhooks/app/token" && r.URL.RawQuery == "wait=true":
			var payload types.MessageCreateParams
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Content != "follow up" {
				t.Fatalf("unexpected content %s", payload.Content)
			}
			_ = json.NewEncoder(w).Encode(types.Message{ID: "234", Content: payload.Content})
		case r.Method == http.MethodPatch && r.URL.Path == "/webhooks/app/token/messages/234":
			var payload types.MessageEditParams
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Content != "edited" {
				t.Fatalf("unexpected edit content %s", payload.Content)
			}
			_ = json.NewEncoder(w).Encode(types.Message{ID: "234", Content: payload.Content})
		case r.Method == http.MethodDelete && r.URL.Path == "/webhooks/app/token/messages/234":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	ic := NewInteractionClient(newInteractionTestClient(t, server.URL))

	created, err := ic.CreateFollowupMessage(context.Background(), "app", "token", &types.MessageCreateParams{Content: "follow up"})
	if err != nil {
		t.Fatalf("CreateFollowupMessage error: %v", err)
	}
	if created.ID != "234" {
		t.Fatalf("expected follow-up message ID 234, got %s", created.ID)
	}

	edited, err := ic.EditFollowupMessage(context.Background(), "app", "token", "234", &types.MessageEditParams{Content: "edited"})
	if err != nil {
		t.Fatalf("EditFollowupMessage error: %v", err)
	}
	if edited.Content != "edited" {
		t.Fatalf("expected edited follow-up content, got %s", edited.Content)
	}

	if err := ic.DeleteFollowupMessage(context.Background(), "app", "token", "234"); err != nil {
		t.Fatalf("DeleteFollowupMessage error: %v", err)
	}
}

func newInteractionTestClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	c, err := client.New("token",
		client.WithBaseURL(baseURL),
		client.WithHTTPClient(&http.Client{}),
		client.WithRateLimiter(&testTracker{}),
		client.WithStrategy(ratelimit.NewReactiveStrategy()),
	)
	if err != nil {
		t.Fatalf("client.New error: %v", err)
	}
	return c
}

type testTracker struct{}

func (t *testTracker) Wait(ctx context.Context, route string) error { return nil }
func (t *testTracker) Update(route string, headers http.Header)     {}
func (t *testTracker) GetBucket(route string) *ratelimit.Bucket     { return nil }
func (t *testTracker) Clear()                                       {}
