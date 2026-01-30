package cmd

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

type stubPublisher struct {
	envelopes []*redisEnvelope
	err       error
}

func (s *stubPublisher) Publish(_ context.Context, env *redisEnvelope) error {
	if s.err != nil {
		return s.err
	}
	s.envelopes = append(s.envelopes, env)
	return nil
}

func (s *stubPublisher) Close() error { return nil }

func TestServerHandleInteractionValidSignature(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Commands: map[string]handlerRoute{
				"help": {Agent: "claude"},
			},
		},
	}
	srv, priv, publisher := newServerWithConfig(t, cfg)

	payload := map[string]any{
		"type":  2,
		"token": "test-token",
		"id":    "123",
		"data": map[string]any{
			"name": "help",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := signedRequest(t, priv, body)
	rec := httptest.NewRecorder()

	srv.HandleInteraction(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var response types.InteractionResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Type != types.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("expected deferred response, got %d", response.Type)
	}
	if len(publisher.envelopes) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(publisher.envelopes))
	}
	env := publisher.envelopes[0]
	if env.Agent != "claude" {
		t.Fatalf("unexpected agent %s", env.Agent)
	}
	if env.Kind != handlerKindCommand {
		t.Fatalf("unexpected kind %s", env.Kind)
	}
	if env.Key != "help" {
		t.Fatalf("unexpected key %s", env.Key)
	}
}

func TestServerHandlesMultipleAgents(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Commands: map[string]handlerRoute{
				"help": {Agent: "claude"},
				"ping": {Agent: "codex"},
			},
		},
	}
	srv, priv, publisher := newServerWithConfig(t, cfg)

	help := map[string]any{"type": 2, "token": "tok-help", "id": "1", "data": map[string]any{"name": "help"}}
	body, _ := json.Marshal(help)
	req := signedRequest(t, priv, body)
	rec := httptest.NewRecorder()
	srv.HandleInteraction(rec, req)
	if len(publisher.envelopes) == 0 || publisher.envelopes[len(publisher.envelopes)-1].Agent != "claude" {
		t.Fatalf("expected claude envelope, got %#v", publisher.envelopes)
	}

	publisher.envelopes = nil
	ping := map[string]any{"type": 2, "token": "tok-ping", "id": "2", "data": map[string]any{"name": "ping"}}
	body, _ = json.Marshal(ping)
	req = signedRequest(t, priv, body)
	rec = httptest.NewRecorder()
	srv.HandleInteraction(rec, req)
	if len(publisher.envelopes) == 0 || publisher.envelopes[0].Agent != "codex" {
		t.Fatalf("expected codex envelope, got %#v", publisher.envelopes)
	}
}

func TestServerHandleInteractionInvalidSignature(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Commands: map[string]handlerRoute{
				"help": {Agent: "claude"},
			},
		},
	}
	srv, _, publisher := newServerWithConfig(t, cfg)

	body := []byte(`{"type":2,"token":"bad","id":"1","data":{"name":"help"}}`)
	req := httptest.NewRequest(http.MethodPost, "/interactions", bytes.NewReader(body))
	req.Header.Set("X-Signature-Timestamp", time.Now().UTC().Format(time.RFC3339Nano))
	req.Header.Set("X-Signature-Ed25519", "deadbeef")
	rec := httptest.NewRecorder()

	srv.HandleInteraction(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid signature, got %d", rec.Result().StatusCode)
	}
	if len(publisher.envelopes) != 0 {
		t.Fatalf("expected no envelopes, got %d", len(publisher.envelopes))
	}
}

func TestServerComponentHandlerPublishesEnvelope(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Components: map[string]handlerRoute{
				"confirm_primary": {Agent: "codex"},
			},
		},
	}
	srv, priv, publisher := newServerWithConfig(t, cfg)

	body, err := json.Marshal(map[string]any{
		"type":  types.InteractionTypeMessageComponent,
		"token": "component-token",
		"id":    "321",
		"data": map[string]any{
			"custom_id": "confirm_primary",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := signedRequest(t, priv, body)
	rec := httptest.NewRecorder()

	srv.HandleInteraction(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Result().StatusCode)
	}
	if len(publisher.envelopes) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(publisher.envelopes))
	}
	env := publisher.envelopes[0]
	if env.Kind != handlerKindComponent || env.Agent != "codex" || env.Key != "confirm_primary" {
		t.Fatalf("unexpected envelope %+v", env)
	}
}

func TestServerModalHandlerPublishesEnvelope(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Modals: map[string]handlerRoute{
				"feedback_modal": {Agent: "opencode"},
			},
		},
	}
	srv, priv, publisher := newServerWithConfig(t, cfg)

	body, err := json.Marshal(map[string]any{
		"type":  types.InteractionTypeModalSubmit,
		"token": "modal-token",
		"id":    "999",
		"data": map[string]any{
			"custom_id": "feedback_modal",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := signedRequest(t, priv, body)
	rec := httptest.NewRecorder()

	srv.HandleInteraction(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Result().StatusCode)
	}
	if len(publisher.envelopes) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(publisher.envelopes))
	}
	env := publisher.envelopes[0]
	if env.Kind != handlerKindModal || env.Agent != "opencode" || env.Key != "feedback_modal" {
		t.Fatalf("unexpected envelope %+v", env)
	}
}

func TestServerAutocompleteHandlerReturnsChoices(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Timeout: time.Second,
		Handlers: handlerMappings{
			Autocomplete: map[string]handlerRoute{
				"session_find": {
					Choices: []autocompleteChoice{
						{Name: "Recent sessions", Value: "recent"},
					},
				},
			},
		},
	}
	srv, priv, publisher := newServerWithConfig(t, cfg)

	body, err := json.Marshal(map[string]any{
		"type":  types.InteractionTypeApplicationCommandAutocomplete,
		"token": "auto-token",
		"id":    "777",
		"data": map[string]any{
			"name": "session_find",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := signedRequest(t, priv, body)
	rec := httptest.NewRecorder()

	srv.HandleInteraction(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Result().StatusCode)
	}
	if len(publisher.envelopes) != 0 {
		t.Fatalf("expected no redis envelopes for autocomplete, got %d", len(publisher.envelopes))
	}
	var resp types.InteractionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Type != types.InteractionResponseAutocompleteResult {
		t.Fatalf("expected autocomplete response, got %d", resp.Type)
	}
	if resp.Data == nil || len(resp.Data.Choices) != 1 || resp.Data.Choices[0].Name != "Recent sessions" {
		t.Fatalf("unexpected autocomplete payload %+v", resp.Data)
	}
}

func newServerWithConfig(t *testing.T, cfg interactionsConfig) (*interactions.Server, ed25519.PrivateKey, *stubPublisher) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv, err := interactions.NewServer(hex.EncodeToString(pub))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = time.Second
	}
	bindings := collectHandlerBindings(cfg)
	if len(bindings) == 0 {
		t.Fatalf("test requires at least one handler binding")
	}
	publisher := &stubPublisher{}
	if err := registerInteractionHandlers(srv, timeout, publisher, bindings); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	return srv, priv, publisher
}

func signedRequest(t *testing.T, priv ed25519.PrivateKey, body []byte) *http.Request {
	t.Helper()
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	message := append([]byte(timestamp), body...)
	signature := ed25519.Sign(priv, message)

	req := httptest.NewRequest(http.MethodPost, "/interactions", bytes.NewReader(body))
	req.Header.Set("X-Signature-Timestamp", timestamp)
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(signature))
	return req
}
