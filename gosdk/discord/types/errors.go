package types

import (
	"errors"
	"fmt"
)

var (
	// ErrRateLimited indicates a rate limit was hit
	ErrRateLimited = errors.New("rate limited by Discord API")

	// ErrUnauthorized indicates invalid or missing authentication
	ErrUnauthorized = errors.New("unauthorized: invalid or missing token")

	// ErrNotFound indicates the requested resource was not found
	ErrNotFound = errors.New("resource not found")

	// ErrBadRequest indicates invalid request parameters
	ErrBadRequest = errors.New("bad request: invalid parameters")

	// ErrServerError indicates a Discord API server error
	ErrServerError = errors.New("Discord API server error")

	// ErrNetworkError indicates a network/connection error
	ErrNetworkError = errors.New("network error")
)

// APIError represents a Discord API error response
type APIError struct {
	StatusCode int
	Message    string
	Code       int
	Errors     map[string]interface{}
	RetryAfter int // seconds to wait before retry (for rate limits)
}

func (e *APIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("Discord API error %d: %s (retry after %ds)", e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("Discord API error %d: %s", e.StatusCode, e.Message)
}

// Is implements error matching for common error types
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrRateLimited:
		return e.StatusCode == 429
	case ErrUnauthorized:
		return e.StatusCode == 401 || e.StatusCode == 403
	case ErrNotFound:
		return e.StatusCode == 404
	case ErrBadRequest:
		return e.StatusCode == 400
	case ErrServerError:
		return e.StatusCode >= 500 && e.StatusCode < 600
	default:
		return false
	}
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NetworkError wraps network-related errors
type NetworkError struct {
	Op  string // operation that failed (e.g., "dial", "write", "read")
	Err error  // underlying error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Op, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

func (e *NetworkError) Is(target error) bool {
	return target == ErrNetworkError
}
