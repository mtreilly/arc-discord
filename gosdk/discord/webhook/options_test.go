package webhook

import (
	"net/http"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/logger"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	client, err := NewClient("http://example.com", WithHTTPClient(customClient))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.httpClient != customClient {
		t.Errorf("WithHTTPClient() did not set custom client")
	}
}

func TestWithMaxRetries(t *testing.T) {
	client, err := NewClient("http://example.com", WithMaxRetries(5))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.maxRetries != 5 {
		t.Errorf("WithMaxRetries() = %d, want 5", client.maxRetries)
	}
}

func TestWithTimeout(t *testing.T) {
	timeout := 45 * time.Second
	client, err := NewClient("http://example.com", WithTimeout(timeout))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.timeout != timeout {
		t.Errorf("WithTimeout() = %v, want %v", client.timeout, timeout)
	}
}

func TestWithRateLimiter(t *testing.T) {
	customLimiter := ratelimit.NewMemoryTracker()

	client, err := NewClient("http://example.com", WithRateLimiter(customLimiter))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.rateLimiter != customLimiter {
		t.Errorf("WithRateLimiter() did not set custom limiter")
	}
}

func TestWithStrategy(t *testing.T) {
	customStrategy := ratelimit.NewReactiveStrategy()

	client, err := NewClient("http://example.com", WithStrategy(customStrategy))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.strategy != customStrategy {
		t.Errorf("WithStrategy() did not set custom strategy")
	}
}

func TestWithStrategyName(t *testing.T) {
	tests := []struct {
		name         string
		strategyName string
		wantType     string
	}{
		{
			name:         "reactive strategy",
			strategyName: "reactive",
			wantType:     "reactive",
		},
		{
			name:         "proactive strategy",
			strategyName: "proactive",
			wantType:     "proactive",
		},
		{
			name:         "adaptive strategy",
			strategyName: "adaptive",
			wantType:     "adaptive",
		},
		{
			name:         "unknown defaults to adaptive",
			strategyName: "unknown",
			wantType:     "adaptive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("http://example.com", WithStrategyName(tt.strategyName))
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			if client.strategy.Name() != tt.wantType {
				t.Errorf("WithStrategyName(%q) strategy name = %v, want %v",
					tt.strategyName, client.strategy.Name(), tt.wantType)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	customLogger := logger.New(logger.DebugLevel, "json", nil)

	client, err := NewClient("http://example.com", WithLogger(customLogger))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.logger != customLogger {
		t.Errorf("WithLogger() did not set custom logger")
	}
}

func TestCreateStrategy(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"reactive", "reactive"},
		{"proactive", "proactive"},
		{"adaptive", "adaptive"},
		{"unknown", "adaptive"}, // defaults to adaptive
		{"", "adaptive"},        // empty defaults to adaptive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createStrategy(tt.name)
			if strategy.Name() != tt.want {
				t.Errorf("createStrategy(%q) = %v, want %v", tt.name, strategy.Name(), tt.want)
			}
		})
	}
}

func TestBackoffFromSeconds(t *testing.T) {
	tests := []struct {
		seconds int
		want    time.Duration
	}{
		{1, 1 * time.Second},
		{5, 5 * time.Second},
		{60, 60 * time.Second},
		{0, 0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoffFromSeconds(tt.seconds)
			if got != tt.want {
				t.Errorf("backoffFromSeconds(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestMultipleOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	customLogger := logger.New(logger.DebugLevel, "json", nil)
	customLimiter := ratelimit.NewMemoryTracker()

	client, err := NewClient("http://example.com",
		WithHTTPClient(customClient),
		WithMaxRetries(10),
		WithTimeout(60*time.Second),
		WithLogger(customLogger),
		WithRateLimiter(customLimiter),
		WithStrategyName("proactive"),
	)

	if err != nil {
		t.Fatalf("NewClient() with multiple options error = %v", err)
	}

	if client.httpClient != customClient {
		t.Errorf("httpClient not set correctly")
	}
	if client.maxRetries != 10 {
		t.Errorf("maxRetries = %d, want 10", client.maxRetries)
	}
	if client.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", client.timeout)
	}
	if client.logger != customLogger {
		t.Errorf("logger not set correctly")
	}
	if client.rateLimiter != customLimiter {
		t.Errorf("rateLimiter not set correctly")
	}
	if client.strategy.Name() != "proactive" {
		t.Errorf("strategy = %v, want proactive", client.strategy.Name())
	}
}
