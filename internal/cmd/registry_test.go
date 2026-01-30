package cmd

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type mockRedisClient struct {
	setCalls []struct {
		key   string
		value []byte
		ttl   time.Duration
	}
	delCalls []string
	setErr   error
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	payload, _ := value.([]byte)
	m.setCalls = append(m.setCalls, struct {
		key   string
		value []byte
		ttl   time.Duration
	}{key: key, value: append([]byte(nil), payload...), ttl: expiration})
	return redis.NewStatusResult("OK", m.setErr)
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.delCalls = append(m.delCalls, keys...)
	return redis.NewIntResult(int64(len(keys)), nil)
}

func (m *mockRedisClient) Close() error { return nil }

func TestAgentRegistryRegisterAndUnregister(t *testing.T) {
	mock := &mockRedisClient{}
	reg := newAgentRegistryWithClient(mock, time.Minute, "arc:discord:registry")

	info := AgentInfo{
		Agent:        "claude",
		Capabilities: []string{"command:help"},
		Channels:     []string{"arc:discord:agent:claude"},
	}
	ctx := context.Background()
	if err := reg.Register(ctx, info); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(mock.setCalls) != 1 {
		t.Fatalf("expected 1 set call, got %d", len(mock.setCalls))
	}
	call := mock.setCalls[0]
	if call.key != "arc:discord:registry:claude" {
		t.Fatalf("unexpected key %s", call.key)
	}
	if call.ttl != time.Minute {
		t.Fatalf("unexpected ttl %v", call.ttl)
	}
	var stored AgentInfo
	if err := json.Unmarshal(call.value, &stored); err != nil {
		t.Fatalf("unmarshal stored: %v", err)
	}
	if stored.Agent != "claude" {
		t.Fatalf("unexpected agent %s", stored.Agent)
	}

	if err := reg.Unregister(ctx, "claude"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if len(mock.delCalls) != 1 || mock.delCalls[0] != "arc:discord:registry:claude" {
		t.Fatalf("unexpected del calls %#v", mock.delCalls)
	}
}

func TestResolveAgentCapabilities(t *testing.T) {
	mappings := handlerMappings{
		Commands: map[string]handlerRoute{
			"help": {Agent: "CLAUDE"},
		},
		Components: map[string]handlerRoute{
			"confirm": {Agent: "claude"},
		},
		Modals: map[string]handlerRoute{
			"feedback": {Agent: "other"},
		},
		Autocomplete: map[string]handlerRoute{
			"session_find": {Agent: "claude"},
		},
	}
	caps := resolveAgentCapabilities("claude", mappings)
	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}
	if caps[0] != "autocomplete:session_find" || caps[1] != "command:help" || caps[2] != "component:confirm" {
		t.Fatalf("unexpected capabilities %#v", caps)
	}
}

func TestAgentRegistryRegisterValidatesAgent(t *testing.T) {
	mock := &mockRedisClient{}
	reg := newAgentRegistryWithClient(mock, time.Minute, "arc:discord:registry")
	if err := reg.Register(context.Background(), AgentInfo{}); err == nil {
		t.Fatalf("expected error when agent missing")
	}
}

func TestAgentRegistryHeartbeat(t *testing.T) {
	mock := &mockRedisClient{}
	reg := newAgentRegistryWithClient(mock, time.Second, "arc:discord:registry")
	info := AgentInfo{Agent: "claude"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- reg.Heartbeat(ctx, info, 5*time.Millisecond) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("heartbeat did not return")
	}
	if len(mock.setCalls) < 2 {
		t.Fatalf("expected multiple Register calls, got %d", len(mock.setCalls))
	}
}
