package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/logger"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

const (
	defaultBaseURL   = "https://discord.com/api"
	defaultUserAgent = "DiscordGoSDK/0.1 (+https://github.com/yourusername/agent-discord)"
)

// PoolConfig adjusts HTTP transport pooling behavior.
type PoolConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	ExpectContinueTimeout time.Duration
}

type poolStats struct {
	totalRequests     int64
	reusedConnections int64
}

func (ps *poolStats) record(reused bool) {
	atomic.AddInt64(&ps.totalRequests, 1)
	if reused {
		atomic.AddInt64(&ps.reusedConnections, 1)
	}
}

// PoolStats exposes connection pool metrics.
type PoolStats struct {
	TotalRequests     int64
	ReusedConnections int64
}

func defaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// Client provides authenticated access to the Discord REST API for bot workflows.
// It mirrors the webhook client's patterns: typed errors, structured logging,
// shared rate-limit tracking, and context-aware requests.
type Client struct {
	token       string
	baseURL     string
	httpClient  *http.Client
	logger      *logger.Logger
	rateLimiter ratelimit.Tracker
	strategy    ratelimit.Strategy
	maxRetries  int
	timeout     time.Duration
	poolConfig  PoolConfig
	poolStats   *poolStats

	middlewares []Middleware
}

// Option customises the bot HTTP client.
type Option func(*Client)

// WithHTTPClient injects a custom http.Client (timeouts are respected if set).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithLogger injects a custom logger.
func WithLogger(l *logger.Logger) Option {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithBaseURL overrides the Discord API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		if url != "" {
			c.baseURL = strings.TrimRight(url, "/")
		}
	}
}

// WithRateLimiter injects a custom rate limiter instance.
func WithRateLimiter(rl ratelimit.Tracker) Option {
	return func(c *Client) {
		if rl != nil {
			c.rateLimiter = rl
		}
	}
}

// WithStrategy injects a custom rate limit strategy.
func WithStrategy(strategy ratelimit.Strategy) Option {
	return func(c *Client) {
		if strategy != nil {
			c.strategy = strategy
		}
	}
}

// WithStrategyName selects a rate limiting strategy by name.
func WithStrategyName(name string) Option {
	return func(c *Client) {
		c.strategy = createStrategy(name)
	}
}

// WithMaxRetries sets the maximum number of retry attempts for failed requests.
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		if retries >= 0 {
			c.maxRetries = retries
		}
	}
}

// WithTimeout overrides the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithPoolConfig overrides the HTTP connection pooling settings.
func WithPoolConfig(cfg PoolConfig) Option {
	return func(c *Client) {
		c.poolConfig = cfg
	}
}

// New creates a new Discord bot HTTP client.
func New(token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, &types.ValidationError{
			Field:   "token",
			Message: "bot token is required",
		}
	}

	c := &Client{
		token:       token,
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{},
		logger:      logger.Default(),
		rateLimiter: ratelimit.NewMemoryTracker(),
		strategy:    ratelimit.NewDefaultAdaptiveStrategy(),
		maxRetries:  3,
		timeout:     30 * time.Second,
		poolConfig:  defaultPoolConfig(),
		poolStats:   &poolStats{},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.configureHTTPClient()

	return c, nil
}

func (c *Client) configureHTTPClient() {
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	if c.httpClient.Transport == nil {
		c.httpClient.Transport = newPooledTransport(c.poolConfig)
	} else if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		applyPoolConfig(t, c.poolConfig)
	}
	if c.httpClient.Timeout == 0 {
		c.httpClient.Timeout = c.timeout
	}
}

func newPooledTransport(cfg PoolConfig) *http.Transport {
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 20
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}
	if cfg.ExpectContinueTimeout <= 0 {
		cfg.ExpectContinueTimeout = time.Second
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
	}
}

func applyPoolConfig(t *http.Transport, cfg PoolConfig) {
	if cfg.MaxIdleConns > 0 {
		t.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost > 0 {
		t.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}
	if cfg.IdleConnTimeout > 0 {
		t.IdleConnTimeout = cfg.IdleConnTimeout
	}
	if cfg.ExpectContinueTimeout > 0 {
		t.ExpectContinueTimeout = cfg.ExpectContinueTimeout
	}
}

// Get performs a GET request relative to the Discord API base path.
func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out, nil)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out, nil)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, out, nil)
}

// Patch performs a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPatch, path, body, out, nil)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}, headers http.Header) error {
	route := c.buildRoute(method, path)
	url := c.buildURL(path)

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	backoff := time.Second
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		if err := c.waitForRateLimit(ctx, route); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}

		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		if c.poolStats != nil {
			trace := &httptrace.ClientTrace{
				GotConn: func(info httptrace.GotConnInfo) {
					c.poolStats.record(info.Reused)
				},
			}
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
		}

		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bot "+c.token)
		req.Header.Set("User-Agent", defaultUserAgent)
		for key, values := range headers {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}

		start := time.Now()
		c.logger.Debug("discord.client.request",
			"method", method,
			"path", path,
			"attempt", attempt+1,
		)

		resp, err := c.execute(ctx, &Request{Request: req})
		if err != nil {
			lastErr = &types.NetworkError{Op: "request", Err: err}
			continue
		}

		if c.rateLimiter != nil {
			c.rateLimiter.Update(route, resp.Header)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.recordStrategyOutcome(route, false)

			if out != nil && resp.Body != nil && resp.ContentLength != 0 {
				defer resp.Body.Close()
				if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
					return fmt.Errorf("failed to decode response: %w", err)
				}
			} else {
				resp.Body.Close()
			}

			c.logger.Debug("discord.client.response",
				"method", method,
				"path", path,
				"status", resp.StatusCode,
				"duration_ms", time.Since(start).Milliseconds(),
			)

			return nil
		}

		apiErr := c.parseErrorResponse(resp)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			c.logger.Warn("rate limit hit",
				"route", route,
				"retry_after", apiErr.RetryAfter,
				"attempt", attempt+1,
			)
			c.recordStrategyOutcome(route, true)

			if apiErr.RetryAfter > 0 {
				backoff = time.Duration(apiErr.RetryAfter) * time.Second
			}
			lastErr = apiErr
			continue
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return apiErr
		}

		lastErr = apiErr
	}

	if lastErr != nil {
		return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return fmt.Errorf("request failed after %d attempts", c.maxRetries+1)
}

func (c *Client) buildURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

func (c *Client) buildRoute(method, path string) string {
	return ratelimit.RouteFromEndpoint(method, c.buildURL(path))
}

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
				)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(waitDuration):
				}
			}
		}
	} else {
		strategyName = "none"
	}

	if err := c.rateLimiter.Wait(ctx, route); err != nil {
		return err
	}

	c.logger.Debug("rate limit: wait complete",
		"route", route,
		"strategy", strategyName,
	)
	return nil
}

func (c *Client) recordStrategyOutcome(route string, hitLimit bool) {
	if adaptive, ok := c.strategy.(*ratelimit.AdaptiveStrategy); ok {
		bucket := c.rateLimiter.GetBucket(route)
		adaptive.RecordRequest(bucket, hitLimit)
	}
}

// PoolStats returns connection pooling metrics for the HTTP client.
func (c *Client) PoolStats() PoolStats {
	if c.poolStats == nil {
		return PoolStats{}
	}
	return PoolStats{
		TotalRequests:     atomic.LoadInt64(&c.poolStats.totalRequests),
		ReusedConnections: atomic.LoadInt64(&c.poolStats.reusedConnections),
	}
}

// execute runs the HTTP request through middleware, falling back to the base transport.
func (c *Client) execute(ctx context.Context, req *Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	handler := c.baseHandler()
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i](handler)
	}
	return handler(req)
}

func (c *Client) baseHandler() RequestHandler {
	return func(req *Request) (*http.Response, error) {
		return c.httpClient.Do(req.Request)
	}
}

// Use registers one or more middleware in FIFO order (first added, first executed).
func (c *Client) Use(mw ...Middleware) {
	c.middlewares = append(c.middlewares, mw...)
}

func (c *Client) parseErrorResponse(resp *http.Response) *types.APIError {
	data, _ := io.ReadAll(resp.Body)
	apiErr := &types.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(data),
	}

	var payload struct {
		Message    string                 `json:"message"`
		Code       int                    `json:"code"`
		Errors     map[string]interface{} `json:"errors"`
		RetryAfter float64                `json:"retry_after"`
	}

	if err := json.Unmarshal(data, &payload); err == nil {
		if payload.Message != "" {
			apiErr.Message = payload.Message
		}
		apiErr.Code = payload.Code
		apiErr.Errors = payload.Errors
		if payload.RetryAfter > 0 {
			apiErr.RetryAfter = int(payload.RetryAfter)
		}
	}

	return apiErr
}

func createStrategy(name string) ratelimit.Strategy {
	switch strings.ToLower(name) {
	case "reactive":
		return ratelimit.NewReactiveStrategy()
	case "proactive":
		return ratelimit.NewDefaultProactiveStrategy()
	case "adaptive":
		return ratelimit.NewDefaultAdaptiveStrategy()
	default:
		return ratelimit.NewDefaultAdaptiveStrategy()
	}
}
