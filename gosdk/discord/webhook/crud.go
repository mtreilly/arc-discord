package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/ratelimit"
)

// MessageEditParams represents parameters for editing a webhook message
type MessageEditParams struct {
	Content         *string        `json:"content,omitempty"`
	Embeds          []types.Embed  `json:"embeds,omitempty"`
	AllowedMentions *struct {
		Parse []string `json:"parse,omitempty"`
	} `json:"allowed_mentions,omitempty"`
	// Note: File attachments cannot be edited, only replaced
}

// Edit edits a previously sent webhook message
// messageID is the ID of the message to edit
func (c *Client) Edit(ctx context.Context, messageID string, params *MessageEditParams) (*types.Message, error) {
	if messageID == "" {
		return nil, &types.ValidationError{
			Field:   "messageID",
			Message: "message ID is required",
		}
	}

	if params == nil {
		return nil, &types.ValidationError{
			Field:   "params",
			Message: "edit parameters are required",
		}
	}

	// Build URL for editing message
	url := c.buildMessageURL(messageID)

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal edit params: %w", err)
	}

	return c.doMessageRequest(ctx, "PATCH", url, body)
}

// Delete deletes a previously sent webhook message
func (c *Client) Delete(ctx context.Context, messageID string) error {
	if messageID == "" {
		return &types.ValidationError{
			Field:   "messageID",
			Message: "message ID is required",
		}
	}

	url := c.buildMessageURL(messageID)
	route := ratelimit.RouteFromEndpoint("DELETE", url)

	var lastErr error
	backoff := c.timeout / 30

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-waitWithBackoff(backoff):
				backoff *= 2
			}
		}

		// Rate limiting
		if err := c.waitForRateLimit(ctx, route); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "DiscordWebhook/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &types.NetworkError{Op: "request", Err: err}
			continue
		}

		// Update rate limiter
		if c.rateLimiter != nil {
			c.rateLimiter.Update(route, resp.Header)
		}

		// Success - 204 No Content
		if resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
			c.recordStrategyOutcome(route, false)
			return nil
		}

		// Parse error
		apiErr := c.parseErrorResponse(resp)
		resp.Body.Close()

		// Handle rate limiting
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limit hit",
				"route", route,
				"retry_after", apiErr.RetryAfter,
				"attempt", attempt+1,
				"method", "DELETE",
			)
			c.recordStrategyOutcome(route, true)

			if apiErr.RetryAfter > 0 {
				backoff = backoffFromSeconds(apiErr.RetryAfter)
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
		return fmt.Errorf("delete request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return fmt.Errorf("delete request failed after %d attempts", c.maxRetries+1)
}

// Get retrieves a previously sent webhook message
func (c *Client) Get(ctx context.Context, messageID string) (*types.Message, error) {
	if messageID == "" {
		return nil, &types.ValidationError{
			Field:   "messageID",
			Message: "message ID is required",
		}
	}

	url := c.buildMessageURL(messageID)

	return c.doMessageRequest(ctx, "GET", url, nil)
}

// buildMessageURL constructs the URL for message operations
func (c *Client) buildMessageURL(messageID string) string {
	// Webhook URLs have the format:
	// https://discord.com/api/webhooks/{webhook.id}/{webhook.token}
	// Message operations append /messages/{message.id}
	return strings.TrimSuffix(c.webhookURL, "/") + "/messages/" + messageID
}

// doMessageRequest performs a request that returns a Message
func (c *Client) doMessageRequest(ctx context.Context, method, url string, body []byte) (*types.Message, error) {
	var lastErr error
	backoff := c.timeout / 30
	route := c.buildRoute(method, url)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-waitWithBackoff(backoff):
				backoff *= 2
			}
		}

		// Rate limiting
		if err := c.waitForRateLimit(ctx, route); err != nil {
			return nil, err
		}

		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "DiscordWebhook/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &types.NetworkError{Op: "request", Err: err}
			continue
		}

		// Update rate limiter
		if c.rateLimiter != nil {
			c.rateLimiter.Update(route, resp.Header)
		}

		// Success - parse response
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()

			var msg types.Message
			if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
				return nil, fmt.Errorf("failed to decode response: %w", err)
			}

			// Record success for adaptive strategy
			c.recordStrategyOutcome(route, false)

			return &msg, nil
		}

		// Parse error
		apiErr := c.parseErrorResponse(resp)
		resp.Body.Close()

		// Handle rate limiting
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limit hit",
				"route", route,
				"retry_after", apiErr.RetryAfter,
				"attempt", attempt+1,
				"method", method,
			)
			c.recordStrategyOutcome(route, true)

			if apiErr.RetryAfter > 0 {
				backoff = backoffFromSeconds(apiErr.RetryAfter)
			}
			lastErr = apiErr
			continue
		}

		// Don't retry client errors (except rate limits)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, apiErr
		}

		// Retry server errors
		lastErr = apiErr
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return nil, fmt.Errorf("request failed after %d attempts", c.maxRetries+1)
}
