package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/logger"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

// Client represents a Discord webhook client
type Client struct {
	webhookURL  string
	httpClient  *http.Client
	maxRetries  int
	timeout     time.Duration
	rateLimiter ratelimit.Tracker
	strategy    ratelimit.Strategy
	logger      *logger.Logger
}

// Option is a functional option for configuring the webhook client
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries sets the maximum number of retry attempts
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		c.maxRetries = retries
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithRateLimiter sets a custom rate limiter
func WithRateLimiter(limiter ratelimit.Tracker) Option {
	return func(c *Client) {
		c.rateLimiter = limiter
	}
}

// WithStrategy sets the rate limiting strategy
func WithStrategy(strategy ratelimit.Strategy) Option {
	return func(c *Client) {
		c.strategy = strategy
	}
}

// WithStrategyName sets the rate limiting strategy by name
// Supported: "reactive", "proactive", "adaptive"
func WithStrategyName(name string) Option {
	return func(c *Client) {
		c.strategy = createStrategy(name)
	}
}

// WithLogger sets a custom logger
func WithLogger(log *logger.Logger) Option {
	return func(c *Client) {
		c.logger = log
	}
}

// NewClient creates a new webhook client
func NewClient(webhookURL string, opts ...Option) (*Client, error) {
	if webhookURL == "" {
		return nil, &types.ValidationError{
			Field:   "webhookURL",
			Message: "webhook URL is required",
		}
	}

	c := &Client{
		webhookURL:  webhookURL,
		httpClient:  &http.Client{},
		maxRetries:  3,
		timeout:     30 * time.Second,
		rateLimiter: ratelimit.NewMemoryTracker(),
		strategy:    ratelimit.NewDefaultAdaptiveStrategy(),
		logger:      logger.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.httpClient.Timeout = c.timeout

	return c, nil
}

// Send sends a message via the webhook
func (c *Client) Send(ctx context.Context, msg *types.WebhookMessage) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid webhook message: %w", err)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook message: %w", err)
	}

	// Build URL with thread_id query parameter if specified
	url := c.buildURLWithThreadID(c.webhookURL, msg.ThreadID)

	return c.sendWithRetryToURL(ctx, body, url)
}

// SendToThread sends a message to a specific thread
func (c *Client) SendToThread(ctx context.Context, threadID string, msg *types.WebhookMessage) error {
	if threadID == "" {
		return &types.ValidationError{
			Field:   "threadID",
			Message: "thread ID is required",
		}
	}

	// Set the thread ID on the message
	msg.ThreadID = threadID

	return c.Send(ctx, msg)
}

// CreateThread creates a new thread with a message (forum channels only)
// Returns error if the webhook is not in a forum channel
func (c *Client) CreateThread(ctx context.Context, threadName string, msg *types.WebhookMessage) error {
	if threadName == "" {
		return &types.ValidationError{
			Field:   "threadName",
			Message: "thread name is required",
		}
	}

	// Set the thread name on the message
	msg.ThreadName = threadName

	return c.Send(ctx, msg)
}

// SendSimple sends a simple text message via the webhook
func (c *Client) SendSimple(ctx context.Context, content string) error {
	return c.Send(ctx, &types.WebhookMessage{
		Content: content,
	})
}

func (c *Client) sendWithRetryToURL(ctx context.Context, body []byte, url string) error {
	var lastErr error
	backoff := time.Second
	route := c.buildRoute("POST", url)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		// Rate limiting: centralize proactive + reactive waits
		if err := c.waitForRateLimit(ctx, route); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "DiscordWebhook/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &types.NetworkError{Op: "request", Err: err}
			continue
		}

		// Update rate limiter with response headers
		if c.rateLimiter != nil {
			c.rateLimiter.Update(route, resp.Header)
		}

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()

			// Record successful request for adaptive strategy
			c.recordStrategyOutcome(route, false)

			return nil
		}

		// Read error response
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		apiErr := &types.APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}

		// Parse JSON error if available
		var errData struct {
			Message    string                 `json:"message"`
			Code       int                    `json:"code"`
			Errors     map[string]interface{} `json:"errors"`
			RetryAfter float64                `json:"retry_after"`
		}
		if json.Unmarshal(respBody, &errData) == nil {
			apiErr.Message = errData.Message
			apiErr.Code = errData.Code
			apiErr.Errors = errData.Errors
			if errData.RetryAfter > 0 {
				apiErr.RetryAfter = int(errData.RetryAfter)
			}
		}

		// Handle rate limiting (429)
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limit hit",
				"route", route,
				"retry_after", apiErr.RetryAfter,
				"attempt", attempt+1,
			)

			// Record rate limit hit for adaptive strategy
			c.recordStrategyOutcome(route, true)

			if apiErr.RetryAfter > 0 {
				backoff = time.Duration(apiErr.RetryAfter) * time.Second
			}
			lastErr = apiErr
			continue
		}

		// Don't retry client errors (except rate limits)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return apiErr
		}

		// Retry server errors
		lastErr = apiErr
	}

	if lastErr != nil {
		return fmt.Errorf("webhook request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return fmt.Errorf("webhook request failed after %d attempts", c.maxRetries+1)
}

// parseErrorResponse parses an HTTP error response into an APIError
func (c *Client) parseErrorResponse(resp *http.Response) *types.APIError {
	respBody, _ := io.ReadAll(resp.Body)

	apiErr := &types.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(respBody),
	}

	// Parse JSON error if available
	var errData struct {
		Message    string                 `json:"message"`
		Code       int                    `json:"code"`
		Errors     map[string]interface{} `json:"errors"`
		RetryAfter float64                `json:"retry_after"`
	}
	if json.Unmarshal(respBody, &errData) == nil {
		apiErr.Message = errData.Message
		apiErr.Code = errData.Code
		apiErr.Errors = errData.Errors
		if errData.RetryAfter > 0 {
			apiErr.RetryAfter = int(errData.RetryAfter)
		}
	}

	return apiErr
}

// marshalJSON marshals a value to JSON
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// waitWithBackoff returns a channel that closes after the specified duration
func waitWithBackoff(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// backoffFromSeconds converts seconds to a duration for backoff
func backoffFromSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// createStrategy creates a rate limiting strategy from a name
func createStrategy(name string) ratelimit.Strategy {
	switch name {
	case "reactive":
		return ratelimit.NewReactiveStrategy()
	case "proactive":
		return ratelimit.NewDefaultProactiveStrategy()
	case "adaptive":
		return ratelimit.NewDefaultAdaptiveStrategy()
	default:
		// Default to adaptive
		return ratelimit.NewDefaultAdaptiveStrategy()
	}
}

// waitForRateLimit handles rate limiting before making a request
func (c *Client) waitForRateLimit(ctx context.Context, route string) error {
	if c.rateLimiter == nil {
		return nil
	}

	var strategyName string
	if c.strategy != nil {
		strategyName = c.strategy.Name()
		bucket := c.rateLimiter.GetBucket(route)
		if bucket != nil && c.strategy.ShouldWait(bucket) {
			waitDuration := c.strategy.CalculateWait(bucket)
			if waitDuration > 0 {
				c.logger.Debug("rate limit: proactive wait",
					"route", route,
					"wait_duration", waitDuration,
					"strategy", strategyName,
					"remaining", bucket.Remaining,
					"limit", bucket.Limit,
				)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(waitDuration):
					c.logger.Debug("rate limit: proactive wait complete",
						"route", route,
						"strategy", strategyName,
					)
				}
			}
		}
	} else {
		strategyName = "none"
	}

	reactiveWait := false
	if bucket := c.rateLimiter.GetBucket(route); bucket != nil && bucket.Remaining <= 0 && time.Now().Before(bucket.Reset) {
		reactiveWait = true
		c.logger.Debug("rate limit: reactive wait scheduled",
			"route", route,
			"reset_in", time.Until(bucket.Reset),
		)
	}

	if err := c.rateLimiter.Wait(ctx, route); err != nil {
		return err
	}

	if reactiveWait {
		c.logger.Debug("rate limit: reactive wait complete",
			"route", route,
			"strategy", strategyName,
		)
	}

	return nil
}

// buildRoute creates a route identifier for rate limiting
func (c *Client) buildRoute(method, url string) string {
	return ratelimit.RouteFromEndpoint(method, url)
}

// recordStrategyOutcome records the outcome of a request for adaptive learning
func (c *Client) recordStrategyOutcome(route string, hitLimit bool) {
	if adaptive, ok := c.strategy.(*ratelimit.AdaptiveStrategy); ok {
		bucket := c.rateLimiter.GetBucket(route)
		adaptive.RecordRequest(bucket, hitLimit)
	}
}

// buildURLWithThreadID builds a URL with the thread_id query parameter if specified
func (c *Client) buildURLWithThreadID(baseURL, threadID string) string {
	if threadID == "" {
		return baseURL
	}
	return baseURL + "?thread_id=" + threadID
}
