package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadInteractionSettingsDefaults(t *testing.T) {
	t.Setenv(envDiscordPublicKey, "ENV_KEY")
	t.Setenv(envTunnelProvider, "auto")
	t.Setenv(envNgrokAuthToken, "token123")
	settings, err := loadInteractionSettings("")
	if err != nil {
		t.Fatalf("loadInteractionSettings: %v", err)
	}
	if settings.PublicKey != "ENV_KEY" {
		t.Fatalf("expected ENV_KEY, got %s", settings.PublicKey)
	}
	if settings.Server.ListenAddr != defaultListenAddr {
		t.Fatalf("default listen addr mismatch: %s", settings.Server.ListenAddr)
	}
	if settings.Redis.ChannelPrefix != defaultRedisPrefix {
		t.Fatalf("default redis prefix mismatch: %s", settings.Redis.ChannelPrefix)
	}
	if settings.Interactions.Timeout != defaultInteractionTimeout {
		t.Fatalf("default timeout mismatch: %v", settings.Interactions.Timeout)
	}
	if settings.Tunnel.Provider != "auto" {
		t.Fatalf("expected env tunnel provider auto, got %s", settings.Tunnel.Provider)
	}
	if settings.Tunnel.NgrokAuthToken != "token123" {
		t.Fatalf("expected env ngrok token, got %s", settings.Tunnel.NgrokAuthToken)
	}
	if len(settings.Interactions.Handlers.Commands) != 0 {
		t.Fatalf("expected no default handlers, got %d", len(settings.Interactions.Handlers.Commands))
	}
}

func TestLoadInteractionSettingsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discord.yaml")
	yaml := `
discord:
  public_key: FILE_KEY

server:
  listen_addr: ":9000"

redis:
  addr: "redis.internal:6380"
  channel_prefix: "custom"

interactions:
  timeout: 30s
  handlers:
    commands:
      help:
        agent: claude
    autocomplete:
      session_find:
        choices:
          - name: "recent"
            value: "recent"
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	settings, err := loadInteractionSettings(path)
	if err != nil {
		t.Fatalf("loadInteractionSettings: %v", err)
	}
	if settings.PublicKey != "FILE_KEY" {
		t.Fatalf("expected FILE_KEY, got %s", settings.PublicKey)
	}
	if settings.Server.ListenAddr != ":9000" {
		t.Fatalf("listen addr mismatch: %s", settings.Server.ListenAddr)
	}
	if settings.Redis.Addr != "redis.internal:6380" {
		t.Fatalf("redis addr mismatch: %s", settings.Redis.Addr)
	}
	if settings.Redis.ChannelPrefix != "custom" {
		t.Fatalf("redis prefix mismatch: %s", settings.Redis.ChannelPrefix)
	}
	if settings.Interactions.Timeout != 30*time.Second {
		t.Fatalf("timeout mismatch: %v", settings.Interactions.Timeout)
	}
	if len(settings.Interactions.Handlers.Commands) != 1 {
		t.Fatalf("expected 1 command handler, got %d", len(settings.Interactions.Handlers.Commands))
	}
	if len(settings.Interactions.Handlers.Autocomplete) != 1 {
		t.Fatalf("expected 1 autocomplete handler, got %d", len(settings.Interactions.Handlers.Autocomplete))
	}
}
