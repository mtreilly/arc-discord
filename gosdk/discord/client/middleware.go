package client

import (
	"context"
	"net/http"
	"time"

	"github.com/yourorg/arc-discord/gosdk/logger"
)

// Request wraps http.Request to allow middleware to override context/metadata.
type Request struct {
	*http.Request
	ctx context.Context
}

// Context returns the request context.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return r.Request.Context()
}

// WithContext updates the underlying http.Request context.
func (r *Request) WithContext(ctx context.Context) {
	r.ctx = ctx
	r.Request = r.Request.WithContext(ctx)
}

// RequestHandler processes an HTTP request.
type RequestHandler func(req *Request) (*http.Response, error)

// Middleware wraps a handler (classic onion).
type Middleware func(next RequestHandler) RequestHandler

// LoggingMiddleware emits debug-level logs for request/response pairs.
func LoggingMiddleware(log *logger.Logger) Middleware {
	if log == nil {
		log = logger.Default()
	}
	return func(next RequestHandler) RequestHandler {
		return func(req *Request) (*http.Response, error) {
			start := time.Now()
			log.Debug("discord.client.middleware.request",
				"method", req.Method,
				"url", req.URL.String(),
			)

			resp, err := next(req)

			log.Debug("discord.client.middleware.response",
				"method", req.Method,
				"url", req.URL.String(),
				"status", statusCode(resp),
				"error", err,
				"duration_ms", time.Since(start).Milliseconds(),
			)

			return resp, err
		}
	}
}

// RetryMiddleware retries failed requests based on shouldRetry predicate.
func RetryMiddleware(maxRetries int, shouldRetry func(*http.Response, error) bool) Middleware {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if shouldRetry == nil {
		shouldRetry = func(resp *http.Response, err error) bool {
			if err != nil {
				return true
			}
			if resp == nil {
				return false
			}
			return resp.StatusCode >= 500
		}
	}

	return func(next RequestHandler) RequestHandler {
		return func(req *Request) (*http.Response, error) {
			var lastErr error
			var resp *http.Response
			backoff := time.Second

			for attempt := 0; attempt <= maxRetries; attempt++ {
				resp, lastErr = next(req)
				if !shouldRetry(resp, lastErr) || attempt == maxRetries {
					return resp, lastErr
				}

				timer := time.NewTimer(backoff)
				select {
				case <-req.Context().Done():
					timer.Stop()
					return resp, req.Context().Err()
				case <-timer.C:
				}
				backoff *= 2
			}

			return resp, lastErr
		}
	}
}

// MetricsMiddleware emits metrics via the provided collector.
func MetricsMiddleware(collect func(method, path string, status int, duration time.Duration)) Middleware {
	if collect == nil {
		return func(next RequestHandler) RequestHandler { return next }
	}

	return func(next RequestHandler) RequestHandler {
		return func(req *Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next(req)
			collect(req.Method, req.URL.Path, statusCode(resp), time.Since(start))
			return resp, err
		}
	}
}

// DryRunMiddleware short-circuits non-GET requests when enabled.
func DryRunMiddleware(enabled bool, log *logger.Logger) Middleware {
	return func(next RequestHandler) RequestHandler {
		return func(req *Request) (*http.Response, error) {
			if enabled && req.Method != http.MethodGet {
				if log != nil {
					log.Info("discord.client.dry_run",
						"method", req.Method,
						"url", req.URL.String(),
					)
				}
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Body:       http.NoBody,
				}, nil
			}
			return next(req)
		}
	}
}

func statusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
