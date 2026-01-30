package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	discordconfig "github.com/yourorg/arc-discord/gosdk/config"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

type stubInteractionResponder struct {
	called         bool
	application    string
	token          string
	params         *types.MessageEditParams
	err            error
	followupCalled bool
	followupParams *types.MessageCreateParams
	followupErr    error
}

func (s *stubInteractionResponder) EditOriginalInteractionResponse(ctx context.Context, applicationID, token string, params *types.MessageEditParams) (*types.Message, error) {
	s.called = true
	s.application = applicationID
	s.token = token
	s.params = params
	return &types.Message{ID: "123"}, s.err
}

func (s *stubInteractionResponder) CreateFollowupMessage(ctx context.Context, applicationID, token string, params *types.MessageCreateParams) (*types.Message, error) {
	s.followupCalled = true
	s.application = applicationID
	s.token = token
	s.followupParams = params
	return &types.Message{ID: "456"}, s.followupErr
}

func mustEnvelope(t *testing.T, env *redisEnvelope) []byte {
	t.Helper()
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return data
}

func TestAgentListenerHandlePayloadEditsResponse(t *testing.T) {
	responder := &stubInteractionResponder{}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})
	interaction := types.Interaction{Token: "tok", Type: types.InteractionTypeApplicationCommand}
	raw, _ := json.Marshal(interaction)
	env := &redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw}

	if err := listener.handlePayload(context.Background(), mustEnvelope(t, env)); err != nil {
		t.Fatalf("handlePayload: %v", err)
	}
	if !responder.called {
		t.Fatalf("expected EditOriginalInteractionResponse to be called")
	}
	if responder.application != "app123" || responder.token != "tok" {
		t.Fatalf("unexpected app/token %s/%s", responder.application, responder.token)
	}
	if responder.params == nil || responder.params.Content == "" {
		t.Fatalf("expected content to be populated")
	}
	if !responder.followupCalled {
		t.Fatalf("expected followup message to be sent")
	}
	if responder.followupParams == nil || !strings.Contains(responder.followupParams.Content, "Follow-up") {
		t.Fatalf("unexpected followup params %+v", responder.followupParams)
	}
}

func TestAgentListenerHandlePayloadSkipsOtherAgent(t *testing.T) {
	responder := &stubInteractionResponder{}
	listener := newAgentListener("codex", "app123", responder, testPrinter{t})
	interaction := types.Interaction{Token: "tok"}
	raw, _ := json.Marshal(interaction)
	env := &redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw}

	if err := listener.handlePayload(context.Background(), mustEnvelope(t, env)); err != nil {
		t.Fatalf("handlePayload: %v", err)
	}
	if responder.called {
		t.Fatalf("expected no call for other agent")
	}
}

func TestAgentListenerHandlePayloadErrorOnMissingToken(t *testing.T) {
	responder := &stubInteractionResponder{}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})
	interaction := types.Interaction{}
	raw, _ := json.Marshal(interaction)
	env := &redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw}

	err := listener.handlePayload(context.Background(), mustEnvelope(t, env))
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestAgentListenerFollowupError(t *testing.T) {
	responder := &stubInteractionResponder{followupErr: errors.New("followup failed")}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})
	interaction := types.Interaction{Token: "tok"}
	raw, _ := json.Marshal(interaction)
	env := &redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw}

	err := listener.handlePayload(context.Background(), mustEnvelope(t, env))
	if err == nil || !strings.Contains(err.Error(), "followup") {
		t.Fatalf("expected followup error, got %v", err)
	}
}

func TestAgentListenerHandlePayloadResponderError(t *testing.T) {
	stubErr := errors.New("edit failed")
	responder := &stubInteractionResponder{err: stubErr}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})
	interaction := types.Interaction{Token: "tok"}
	raw, _ := json.Marshal(interaction)
	env := &redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw}

	err := listener.handlePayload(context.Background(), mustEnvelope(t, env))
	if !errors.Is(err, stubErr) {
		t.Fatalf("expected responder error, got %v", err)
	}
}

func TestAgentListenerHandlePayloadInvalidJSON(t *testing.T) {
	responder := &stubInteractionResponder{}
	listener := newAgentListener("claude", "app123", responder, testPrinter{t})

	if err := listener.handlePayload(context.Background(), []byte("not-json")); err != nil {
		t.Fatalf("expected graceful handling of invalid JSON, got %v", err)
	}
	if responder.called {
		t.Fatalf("expected no responder call on invalid payload")
	}
}

// testPrinter satisfies outputPrinter for tests.
type testPrinter struct{ t *testing.T }

func (tp testPrinter) Printf(format string, args ...interface{}) { tp.t.Logf(format, args...) }

type stubInteractionSubscriber struct {
	payload []byte
	closed  bool
	err     error
}

func (s *stubInteractionSubscriber) Subscribe(ctx context.Context, handler func(context.Context, []byte) error) error {
	if handler != nil && len(s.payload) > 0 {
		if err := handler(ctx, s.payload); err != nil {
			return err
		}
	}
	return s.err
}

func (s *stubInteractionSubscriber) Close() error {
	s.closed = true
	return nil
}

type stubRegistry struct {
	registered   int
	unregistered int
	closed       bool
}

func (s *stubRegistry) Register(ctx context.Context, info AgentInfo) error {
	s.registered++
	return nil
}

func (s *stubRegistry) Heartbeat(ctx context.Context, info AgentInfo, interval time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *stubRegistry) Unregister(ctx context.Context, agent string) error {
	s.unregistered++
	return nil
}

func (s *stubRegistry) Close() error {
	s.closed = true
	return nil
}

func TestRunAgentListenRegistersAndExits(t *testing.T) {
	dir := t.TempDir()
	config := `discord:
  bot_token: dummy
  application_id: "app123"
interactions:
  enabled: true
  handlers:
    commands:
      help:
        agent: claude
`
	path := filepath.Join(dir, "discord.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	interaction := types.Interaction{Token: "tok"}
	raw, _ := json.Marshal(interaction)
	payload, _ := json.Marshal(&redisEnvelope{Agent: "claude", Kind: handlerKindCommand, Key: "help", Interaction: raw})
	stubSub := &stubInteractionSubscriber{payload: payload}
	newRedisSubscriberFn = func(cfg redisConfig, agent string) (interactionSubscriber, error) { return stubSub, nil }
	t.Cleanup(func() { newRedisSubscriberFn = newRedisSubscriber })
	responder := &stubInteractionResponder{}
	newInteractionClientFn = func(cfg *discordconfig.Config, token string) (interactionResponder, error) { return responder, nil }
	t.Cleanup(func() { newInteractionClientFn = createInteractionClient })
	reg := &stubRegistry{}
	newAgentRegistryFn = func(cfg redisConfig, ttl time.Duration) (agentRegistryClient, error) { return reg, nil }
	t.Cleanup(func() {
		newAgentRegistryFn = func(cfg redisConfig, ttl time.Duration) (agentRegistryClient, error) {
			return newAgentRegistry(cfg, ttl)
		}
	})
	cmd := &cobra.Command{}
	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)
	opts := &globalOptions{configPath: path}
	done := make(chan error, 1)
	go func() {
		done <- runAgentListen(cmd, opts, agentListenOptions{AgentID: "claude"})
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAgentListen: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("runAgentListen did not return")
	}
	if !stubSub.closed {
		t.Fatalf("expected subscriber close")
	}
	if reg.registered == 0 || reg.unregistered == 0 || !reg.closed {
		t.Fatalf("registry not cleaned up: %+v", reg)
	}
	if !responder.called {
		t.Fatalf("expected interaction responder call")
	}
}
