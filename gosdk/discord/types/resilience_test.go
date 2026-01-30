package types

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerTrip(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	err := cb.Call(func() error { return errors.New("fail") })
	if err == nil {
		t.Fatal("expected error")
	}
	err = cb.Call(func() error { return errors.New("fail") })
	if err == nil {
		t.Fatal("expected error during trip")
	}
	err = cb.Call(func() error { return nil })
	if err == nil {
		t.Fatalf("expected breaker open error")
	}
	time.Sleep(60 * time.Millisecond)
	err = cb.Call(func() error { return nil })
	if err != nil {
		t.Fatalf("expected recover, got %v", err)
	}
}

func TestRetryPolicy(t *testing.T) {
	rp := &RetryPolicy{MaxAttempts: 3, BackoffBase: 1 * time.Millisecond, BackoffMax: 3 * time.Millisecond}
	var attempts int
	err := rp.Execute(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("unexpected attempts %d", attempts)
	}
}
