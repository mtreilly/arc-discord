package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/client"
)

const defaultGatewayURL = "https://discord.com/api/gateway"

// Checker performs health checks against Discord endpoints.
type Checker struct {
	apiClient  *client.Client
	httpClient *http.Client
	gatewayURL string
}

// NewChecker builds a health checker.
func NewChecker(apiClient *client.Client, opts ...Option) *Checker {
	h := &Checker{
		apiClient:  apiClient,
		httpClient: http.DefaultClient,
		gatewayURL: defaultGatewayURL,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Option configures the health checker.
type Option func(*Checker)

// WithHTTPClient overrides the HTTP client used for gateway/webhook checks.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(h *Checker) {
		if httpClient != nil {
			h.httpClient = httpClient
		}
	}
}

// WithGatewayURL overrides the gateway URL used by CheckGateway.
func WithGatewayURL(url string) Option {
	return func(h *Checker) {
		if url != "" {
			h.gatewayURL = url
		}
	}
}

// CheckAPI validates the REST API by hitting /gateway/bot.
func (h *Checker) CheckAPI(ctx context.Context) error {
	if h.apiClient == nil {
		return errors.New("api client is not configured")
	}
	var resp map[string]interface{}
	return h.apiClient.Get(ctx, "/gateway/bot", &resp)
}

// CheckGateway validates the gateway endpoint is reachable.
func (h *Checker) CheckGateway(ctx context.Context) error {
	if h.httpClient == nil {
		return errors.New("http client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.gatewayURL, nil)
	if err != nil {
		return err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway check failed with status %d", resp.StatusCode)
	}
	return nil
}

// CheckWebhook validates a webhook URL by issuing a GET request.
func (h *Checker) CheckWebhook(ctx context.Context, webhookURL string) error {
	if webhookURL == "" {
		return errors.New("webhook URL is required")
	}
	if h.httpClient == nil {
		return errors.New("http client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, webhookURL, nil)
	if err != nil {
		return err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook check failed with status %d", resp.StatusCode)
}

// HealthReport summarizes the results of the checks.
type HealthReport struct {
	Timestamp time.Time         `json:"timestamp"`
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
}

// Report executes everything and returns a consolidated status.
func (h *Checker) Report(ctx context.Context, webhookURL string) (*HealthReport, error) {
	checks := map[string]string{}
	status := "ok"

	if err := h.CheckAPI(ctx); err != nil {
		checks["api"] = err.Error()
		status = "degraded"
	} else {
		checks["api"] = "ok"
	}

	if err := h.CheckGateway(ctx); err != nil {
		checks["gateway"] = err.Error()
		status = "degraded"
	} else {
		checks["gateway"] = "ok"
	}

	if webhookURL != "" {
		if err := h.CheckWebhook(ctx, webhookURL); err != nil {
			checks["webhook"] = err.Error()
			status = "degraded"
		} else {
			checks["webhook"] = "ok"
		}
	}

	return &HealthReport{
		Timestamp: time.Now().UTC(),
		Status:    status,
		Checks:    checks,
	}, nil
}
