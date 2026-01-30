package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/logger"
)

func TestLoggingMiddleware(t *testing.T) {
	var called int32
	log := logger.Default()
	handler := LoggingMiddleware(log)(func(req *Request) (*http.Response, error) {
		atomic.AddInt32(&called, 1)
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	req := &Request{Request: httptest.NewRequest(http.MethodGet, "http://example.com", nil)}
	if _, err := handler(req); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected handler to run once, got %d", called)
	}
}

func TestRetryMiddlewareRetriesErrors(t *testing.T) {
	var attempts int32

	handler := RetryMiddleware(2, nil)(func(req *Request) (*http.Response, error) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			return nil, errors.New("boom")
		}
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	req := &Request{Request: httptest.NewRequest(http.MethodGet, "http://example.com", nil)}
	req.WithContext(context.Background())

	if _, err := handler(req); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	var recorded int32
	handler := MetricsMiddleware(func(method, path string, status int, duration time.Duration) {
		if method == http.MethodGet && path == "/test" && status == http.StatusOK {
			atomic.AddInt32(&recorded, 1)
		}
	})(func(req *Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	req := &Request{Request: httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)}
	req.WithContext(context.Background())

	if _, err := handler(req); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if recorded != 1 {
		t.Fatalf("expected metrics to record once, got %d", recorded)
	}
}

func TestDryRunMiddleware(t *testing.T) {
	handler := DryRunMiddleware(true, logger.Default())(func(req *Request) (*http.Response, error) {
		t.Fatalf("handler should not be called during dry run")
		return nil, nil
	})

	req := &Request{Request: httptest.NewRequest(http.MethodPost, "http://example.com", nil)}
	req.WithContext(context.Background())

	resp, err := handler(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 status, got %d", resp.StatusCode)
	}
}
