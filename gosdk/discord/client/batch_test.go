package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBatcherFlushesRequests(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cl, err := New("token", WithBaseURL(server.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New client failed: %v", err)
	}
	batcher := cl.NewBatcher(WithBatchSize(2), WithFlushInterval(50*time.Millisecond))
	defer batcher.Stop()

	if err := batcher.AddMessage(context.Background(), "channel", "hi"); err != nil {
		t.Fatalf("AddMessage error: %v", err)
	}
	if err := batcher.AddReaction(context.Background(), "channel", "msg", "emoji"); err != nil {
		t.Fatalf("AddReaction error: %v", err)
	}
	if err := batcher.Flush(context.Background()); err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
}
