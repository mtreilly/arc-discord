package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

func TestNewClientRequiresToken(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	var vErr *types.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestClientGetSuccess(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123","name":"test"}`))
	}))
	defer server.Close()

	client, err := New("test-token",
		WithBaseURL(server.URL),
		WithRateLimiter(&noopTracker{}),
		WithStrategy(ratelimit.NewReactiveStrategy()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var resp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := client.Get(context.Background(), "/channels/123", &resp); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if receivedAuth != "Bot test-token" {
		t.Fatalf("expected auth header, got %s", receivedAuth)
	}
	if resp.ID != "123" || resp.Name != "test" {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestClientRetriesOnServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New("token",
		WithBaseURL(server.URL),
		WithRateLimiter(&noopTracker{}),
		WithStrategy(ratelimit.NewReactiveStrategy()),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := client.Post(context.Background(), "/test", map[string]string{"foo": "bar"}, nil); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"bad","code":100}`))
	}))
	defer server.Close()

	client, err := New("token",
		WithBaseURL(server.URL),
		WithRateLimiter(&noopTracker{}),
		WithStrategy(ratelimit.NewReactiveStrategy()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.Post(context.Background(), "/fail", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if !errors.Is(err, types.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestClientRespectsContextCancellation(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New("token",
		WithBaseURL(server.URL),
		WithRateLimiter(&noopTracker{}),
		WithStrategy(ratelimit.NewReactiveStrategy()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.Get(ctx, "/test", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	close(block)
}

func TestClientWaitsOnRateLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	route := "GET:" + server.URL + "/test"
	tracker := &mockTracker{
		buckets: map[string]*ratelimit.Bucket{
			route: {
				Limit:     5,
				Remaining: 0,
				Reset:     time.Now().Add(10 * time.Millisecond),
			},
		},
	}

	client, err := New("token",
		WithBaseURL(server.URL),
		WithRateLimiter(tracker),
		WithStrategy(ratelimit.NewProactiveStrategy(1, 0)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := client.Get(context.Background(), "/test", nil); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tracker.waits != 1 {
		t.Fatalf("expected wait to be called, got %d", tracker.waits)
	}
}

// --- helpers ---

type noopTracker struct{}

func (n *noopTracker) Wait(ctx context.Context, route string) error { return nil }
func (n *noopTracker) Update(route string, headers http.Header)     {}
func (n *noopTracker) GetBucket(route string) *ratelimit.Bucket     { return nil }
func (n *noopTracker) Clear()                                       {}

type mockTracker struct {
	waitCalled []string
	buckets    map[string]*ratelimit.Bucket
	waits      int
}

func (m *mockTracker) Wait(ctx context.Context, route string) error {
	m.waits++
	m.waitCalled = append(m.waitCalled, route)
	return nil
}

func (m *mockTracker) Update(route string, headers http.Header) {}

func (m *mockTracker) GetBucket(route string) *ratelimit.Bucket {
	if m.buckets == nil {
		return nil
	}
	return m.buckets[route]
}

func (m *mockTracker) Clear() {}
