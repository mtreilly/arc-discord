package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigRateLimit(t *testing.T) {
	t.Setenv("DISCORD_RATE_LIMIT_STRATEGY", "")

	cfg := Default()
	if cfg.Client.RateLimit.Strategy != "adaptive" {
		t.Fatalf("expected default strategy adaptive, got %s", cfg.Client.RateLimit.Strategy)
	}
	if cfg.Client.RateLimit.BackoffBase != time.Second {
		t.Fatalf("expected backoff base 1s, got %v", cfg.Client.RateLimit.BackoffBase)
	}
	if cfg.Client.RateLimit.BackoffMax != 60*time.Second {
		t.Fatalf("expected backoff max 60s, got %v", cfg.Client.RateLimit.BackoffMax)
	}
}

func TestLoadLegacyRateLimitStrategy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
client:
  timeout: 5s
  rate_limit_strategy: proactive
`), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Client.RateLimit.Strategy != "proactive" {
		t.Fatalf("expected strategy proactive, got %s", cfg.Client.RateLimit.Strategy)
	}
}

func TestLoadRateLimitBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
client:
  rate_limit:
    strategy: reactive
    backoff_base: 2s
    backoff_max: 10s
`), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Client.RateLimit.Strategy != "reactive" {
		t.Fatalf("expected strategy reactive, got %s", cfg.Client.RateLimit.Strategy)
	}
	if cfg.Client.RateLimit.BackoffBase != 2*time.Second {
		t.Fatalf("expected backoff base 2s, got %v", cfg.Client.RateLimit.BackoffBase)
	}
	if cfg.Client.RateLimit.BackoffMax != 10*time.Second {
		t.Fatalf("expected backoff max 10s, got %v", cfg.Client.RateLimit.BackoffMax)
	}
}
