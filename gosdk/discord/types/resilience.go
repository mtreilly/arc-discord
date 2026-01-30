package types

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	stateClosed   = "closed"
	stateOpen     = "open"
	stateHalfOpen = "half-open"
)

// CircuitBreaker protects unstable dependencies by short-circuiting calls.
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	mu           sync.Mutex

	state       string
	failures    int
	lastFailure time.Time
}

// NewCircuitBreaker constructs a breaker with the given params.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 5 * time.Second
	}
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        stateClosed,
	}
}

// Call executes fn unless the breaker is open.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == stateOpen && time.Since(cb.lastFailure) < cb.resetTimeout {
		cb.mu.Unlock()
		return errors.New("circuit breaker is open")
	}
	if cb.state == stateOpen {
		cb.state = stateHalfOpen
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err == nil {
		cb.state = stateClosed
		cb.failures = 0
		return nil
	}

	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.maxFailures {
		cb.state = stateOpen
	}
	return err
}

// RetryPolicy defines a retry/backoff strategy.
type RetryPolicy struct {
	MaxAttempts int
	BackoffBase time.Duration
	BackoffMax  time.Duration
	Jitter      bool
}

// Execute runs fn according to the retry policy.
func (rp *RetryPolicy) Execute(ctx context.Context, fn func() error) error {
	if rp.MaxAttempts <= 0 {
		rp.MaxAttempts = 3
	}
	if rp.BackoffBase <= 0 {
		rp.BackoffBase = 200 * time.Millisecond
	}
	if rp.BackoffMax <= 0 {
		rp.BackoffMax = 5 * time.Second
	}

	var attempt int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}
		attempt++
		if attempt >= rp.MaxAttempts {
			return fmt.Errorf("retry policy exhausted: %w", err)
		}

		backoff := rp.BackoffBase << (attempt - 1)
		if backoff > rp.BackoffMax {
			backoff = rp.BackoffMax
		}
		if rp.Jitter {
			backoff = addJitter(backoff)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func addJitter(d time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(d)/2 + 1))
	return d - jitter
}
