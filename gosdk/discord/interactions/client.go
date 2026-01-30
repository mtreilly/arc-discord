package interactions

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// InteractionClient wraps the bot client with helpers for interaction callback and follow-up endpoints.
type InteractionClient struct {
	base *client.Client
}

// NewInteractionClient creates a new interaction client bound to the provided bot client.
func NewInteractionClient(c *client.Client) *InteractionClient {
	if c == nil {
		panic("client.Client is required")
	}
	return &InteractionClient{base: c}
}

// CreateInteractionResponse sends the initial callback payload for an interaction.
func (ic *InteractionClient) CreateInteractionResponse(ctx context.Context, interactionID, token string, resp *types.InteractionResponse) error {
	if err := ensureID("interactionID", interactionID); err != nil {
		return err
	}
	if err := ensureID("token", token); err != nil {
		return err
	}
	if resp == nil {
		return &types.ValidationError{Field: "response", Message: "response payload is required"}
	}
	if err := resp.Validate(); err != nil {
		return err
	}

	path := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, token)
	return ic.base.Post(ctx, path, resp, nil)
}

// GetOriginalInteractionResponse returns the original response message for an interaction.
func (ic *InteractionClient) GetOriginalInteractionResponse(ctx context.Context, applicationID, token string) (*types.Message, error) {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/messages/@original", ic.webhookPath(applicationID, token))
	var msg types.Message
	if err := ic.base.Get(ctx, path, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// EditOriginalInteractionResponse updates the original interaction response.
func (ic *InteractionClient) EditOriginalInteractionResponse(ctx context.Context, applicationID, token string, params *types.MessageEditParams) (*types.Message, error) {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "message edit params are required"}
	}

	path := fmt.Sprintf("%s/messages/@original", ic.webhookPath(applicationID, token))
	var msg types.Message
	if err := ic.base.Patch(ctx, path, params, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteOriginalInteractionResponse removes the original response message.
func (ic *InteractionClient) DeleteOriginalInteractionResponse(ctx context.Context, applicationID, token string) error {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/messages/@original", ic.webhookPath(applicationID, token))
	return ic.base.Delete(ctx, path)
}

// CreateFollowupMessage sends a follow-up message using the webhook endpoint.
func (ic *InteractionClient) CreateFollowupMessage(ctx context.Context, applicationID, token string, params *types.MessageCreateParams) (*types.Message, error) {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "message create params are required"}
	}

	path := ic.webhookPath(applicationID, token) + buildWaitQuery()
	var msg types.Message
	if err := ic.base.Post(ctx, path, params, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// EditFollowupMessage updates an existing follow-up message.
func (ic *InteractionClient) EditFollowupMessage(ctx context.Context, applicationID, token, messageID string, params *types.MessageEditParams) (*types.Message, error) {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return nil, err
	}
	if err := ensureID("messageID", messageID); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "message edit params are required"}
	}

	path := fmt.Sprintf("%s/messages/%s", ic.webhookPath(applicationID, token), messageID)
	var msg types.Message
	if err := ic.base.Patch(ctx, path, params, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteFollowupMessage removes a follow-up message.
func (ic *InteractionClient) DeleteFollowupMessage(ctx context.Context, applicationID, token, messageID string) error {
	if err := ensureAppAndToken(applicationID, token); err != nil {
		return err
	}
	if err := ensureID("messageID", messageID); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/messages/%s", ic.webhookPath(applicationID, token), messageID)
	return ic.base.Delete(ctx, path)
}

func (ic *InteractionClient) webhookPath(applicationID, token string) string {
	return fmt.Sprintf("/webhooks/%s/%s", applicationID, token)
}

func ensureAppAndToken(applicationID, token string) error {
	if err := ensureID("applicationID", applicationID); err != nil {
		return err
	}
	return ensureID("token", token)
}

func ensureID(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return &types.ValidationError{Field: field, Message: "value is required"}
	}
	return nil
}

func buildWaitQuery() string {
	q := url.Values{}
	q.Set("wait", "true")
	return "?" + q.Encode()
}
