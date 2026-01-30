package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Channels exposes channel-related API helpers.
type Channels struct {
	client *Client
}

// Channels returns a channel service bound to the client instance.
func (c *Client) Channels() *Channels {
	return &Channels{client: c}
}

// GetChannel retrieves a channel by ID.
func (c *Channels) GetChannel(ctx context.Context, channelID string) (*types.Channel, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}

	var channel types.Channel
	if err := c.client.Get(ctx, fmt.Sprintf("/channels/%s", channelID), &channel); err != nil {
		return nil, err
	}

	return &channel, nil
}

// ModifyChannel updates channel metadata (topic, position, etc.).
func (c *Channels) ModifyChannel(ctx context.Context, channelID string, params *types.ModifyChannelParams) (*types.Channel, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "modify params required"}
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	headers := http.Header{}
	if params.AuditLogReason != "" {
		headers.Set("X-Audit-Log-Reason", url.QueryEscape(params.AuditLogReason))
	}

	var channel types.Channel
	if err := c.client.do(ctx, http.MethodPatch, fmt.Sprintf("/channels/%s", channelID), params, &channel, headers); err != nil {
		return nil, err
	}
	return &channel, nil
}

// DeleteChannel removes a channel.
func (c *Channels) DeleteChannel(ctx context.Context, channelID string) error {
	if err := validateID("channelID", channelID); err != nil {
		return err
	}
	return c.client.Delete(ctx, fmt.Sprintf("/channels/%s", channelID))
}

// GetChannelMessagesParams controls pagination for channel history.
type GetChannelMessagesParams struct {
	Limit  int
	Before string
	After  string
	Around string
}

// GetChannelMessages fetches channel messages with pagination support.
func (c *Channels) GetChannelMessages(ctx context.Context, channelID string, params *GetChannelMessagesParams) ([]*types.Message, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}

	query := url.Values{}
	if params != nil {
		if err := params.validate(); err != nil {
			return nil, err
		}
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Before != "" {
			query.Set("before", params.Before)
		}
		if params.After != "" {
			query.Set("after", params.After)
		}
		if params.Around != "" {
			query.Set("around", params.Around)
		}
	}

	path := fmt.Sprintf("/channels/%s/messages", channelID)
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var messages []*types.Message
	if err := c.client.Get(ctx, path, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func validateID(field, id string) error {
	if id == "" {
		return &types.ValidationError{Field: field, Message: "ID is required"}
	}
	return nil
}

func (p *GetChannelMessagesParams) validate() error {
	if p == nil {
		return nil
	}
	if p.Limit < 0 {
		return &types.ValidationError{Field: "limit", Message: "limit must be positive"}
	}
	if p.Limit > 0 && (p.Limit < 1 || p.Limit > 100) {
		return &types.ValidationError{Field: "limit", Message: "limit must be between 1 and 100"}
	}
	if p.Around != "" && (p.Before != "" || p.After != "") {
		return &types.ValidationError{Field: "around", Message: "cannot use around with before/after"}
	}
	return nil
}
