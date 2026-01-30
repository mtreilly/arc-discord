package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

type channelPublisher struct {
	ch chan *redisEnvelope
}

func (p *channelPublisher) Publish(ctx context.Context, env *redisEnvelope) error {
	p.ch <- env
	return nil
}

func (p *channelPublisher) Close() error { return nil }

type stubResponder struct {
	edited   bool
	followup bool
}

func (s *stubResponder) EditOriginalInteractionResponse(ctx context.Context, applicationID, token string, params *types.MessageEditParams) (*types.Message, error) {
	s.edited = true
	return &types.Message{ID: "orig"}, nil
}

func (s *stubResponder) CreateFollowupMessage(ctx context.Context, applicationID, token string, params *types.MessageCreateParams) (*types.Message, error) {
	s.followup = true
	return &types.Message{ID: "follow"}, nil
}

func TestIntegrationServerToAgentFlow(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Commands: map[string]handlerRoute{
				"help": {Agent: "claude"},
			},
		},
	}
	publisher := &channelPublisher{ch: make(chan *redisEnvelope, 1)}
	srv, err := interactions.NewServer(strings.Repeat("0", 64), interactions.WithDryRun(true))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	bindings := collectHandlerBindings(cfg)
	if err := registerInteractionHandlers(srv, cfg.Timeout, publisher, bindings); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { srv.HandleInteraction(w, r) }))
	defer ts.Close()

	payload := map[string]any{
		"type":  types.InteractionTypeApplicationCommand,
		"token": "tok",
		"id":    "123",
		"data": map[string]any{
			"name": "help",
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	env := <-publisher.ch
	responder := &stubResponder{}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})
	if err := listener.handlePayload(context.Background(), mustEnvelope(t, env)); err != nil {
		t.Fatalf("handlePayload: %v", err)
	}
	if !responder.edited || !responder.followup {
		t.Fatalf("expected edit and followup to run, got edited=%v followup=%v", responder.edited, responder.followup)
	}
}
