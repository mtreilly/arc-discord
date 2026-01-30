package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// MessageService provides helpers for channel message operations.
type MessageService struct {
	client *Client
}

// Messages exposes message helpers bound to the client.
func (c *Client) Messages() *MessageService {
	return &MessageService{client: c}
}

// Reactions exposes reaction helpers (alias for clarity).
func (c *Client) Reactions() *MessageService {
	return c.Messages()
}

// CreateMessage sends a message to a channel.
func (m *MessageService) CreateMessage(ctx context.Context, channelID string, params *types.MessageCreateParams) (*types.Message, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "message create params required"}
	}

	var msg types.Message
	if err := m.client.Post(ctx, fmt.Sprintf("/channels/%s/messages", channelID), params, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetMessage fetches a single message.
func (m *MessageService) GetMessage(ctx context.Context, channelID, messageID string) (*types.Message, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}
	if err := validateID("messageID", messageID); err != nil {
		return nil, err
	}

	var msg types.Message
	if err := m.client.Get(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// EditMessage updates message content/embeds.
func (m *MessageService) EditMessage(ctx context.Context, channelID, messageID string, params *types.MessageEditParams) (*types.Message, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}
	if err := validateID("messageID", messageID); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "message edit params required"}
	}

	var msg types.Message
	if err := m.client.Patch(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), params, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteMessage removes a single message.
func (m *MessageService) DeleteMessage(ctx context.Context, channelID, messageID string) error {
	if err := validateID("channelID", channelID); err != nil {
		return err
	}
	if err := validateID("messageID", messageID); err != nil {
		return err
	}
	return m.client.Delete(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID))
}

// BulkDeleteMessages deletes multiple messages in one call.
func (m *MessageService) BulkDeleteMessages(ctx context.Context, channelID string, messageIDs []string) error {
	if err := validateID("channelID", channelID); err != nil {
		return err
	}
	if len(messageIDs) == 0 {
		return &types.ValidationError{Field: "messages", Message: "at least one message ID required"}
	}
	if len(messageIDs) > 100 {
		return &types.ValidationError{Field: "messages", Message: "maximum 100 messages per bulk delete"}
	}

	payload := struct {
		Messages []string `json:"messages"`
	}{
		Messages: messageIDs,
	}
	return m.client.Post(ctx, fmt.Sprintf("/channels/%s/messages/bulk-delete", channelID), payload, nil)
}

// CreateReaction adds the current bot's reaction to a message.
func (m *MessageService) CreateReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return m.reactionRequest(ctx, http.MethodPut, channelID, messageID, emoji, "@me", "")
}

// DeleteOwnReaction removes the bot's own reaction from a message.
func (m *MessageService) DeleteOwnReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return m.reactionRequest(ctx, http.MethodDelete, channelID, messageID, emoji, "@me", "")
}

// DeleteUserReaction removes a specific user's reaction.
func (m *MessageService) DeleteUserReaction(ctx context.Context, channelID, messageID, emoji, userID string) error {
	if err := validateID("userID", userID); err != nil {
		return err
	}
	return m.reactionRequest(ctx, http.MethodDelete, channelID, messageID, emoji, userID, "")
}

// DeleteAllReactions removes every reaction from a message (optionally limited to emoji when provided).
func (m *MessageService) DeleteAllReactions(ctx context.Context, channelID, messageID string, emoji string) error {
	if err := validateID("channelID", channelID); err != nil {
		return err
	}
	if err := validateID("messageID", messageID); err != nil {
		return err
	}
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions", channelID, messageID)
	if emoji != "" {
		e, err := encodeEmoji(emoji)
		if err != nil {
			return err
		}
		path = path + "/" + e
	}
	return m.client.Delete(ctx, path)
}

// GetReactions returns a list of users who reacted with the given emoji.
func (m *MessageService) GetReactions(ctx context.Context, channelID, messageID, emoji string, params *GetReactionsParams) ([]*types.User, error) {
	if err := validateID("channelID", channelID); err != nil {
		return nil, err
	}
	if err := validateID("messageID", messageID); err != nil {
		return nil, err
	}
	if emoji == "" {
		return nil, &types.ValidationError{Field: "emoji", Message: "emoji is required"}
	}
	if params != nil {
		if err := params.validate(); err != nil {
			return nil, err
		}
	}

	encoded, err := encodeEmoji(emoji)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", fmt.Sprintf("%d", params.Limit))
		}
		if params.After != "" {
			query.Set("after", params.After)
		}
	}
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s", channelID, messageID, encoded)
	if q := query.Encode(); q != "" {
		path += "?" + q
	}

	var users []*types.User
	if err := m.client.Get(ctx, path, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (m *MessageService) reactionRequest(ctx context.Context, method, channelID, messageID, emoji, suffix, body string) error {
	if err := validateID("channelID", channelID); err != nil {
		return err
	}
	if err := validateID("messageID", messageID); err != nil {
		return err
	}
	if emoji == "" {
		return &types.ValidationError{Field: "emoji", Message: "emoji is required"}
	}
	encoded, err := encodeEmoji(emoji)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/%s", channelID, messageID, encoded, suffix)
	switch method {
	case http.MethodPut:
		return m.client.Put(ctx, path, nil, nil)
	case http.MethodDelete:
		return m.client.Delete(ctx, path)
	default:
		return fmt.Errorf("unsupported reaction method %s", method)
	}
}

// GetReactionsParams controls pagination for reactions.
type GetReactionsParams struct {
	Limit int
	After string
}

func (p *GetReactionsParams) validate() error {
	if p == nil {
		return nil
	}
	if p.Limit < 0 || p.Limit > 100 {
		return &types.ValidationError{Field: "limit", Message: "limit must be between 0 and 100"}
	}
	return nil
}

func encodeEmoji(emoji string) (string, error) {
	if emoji == "" {
		return "", &types.ValidationError{Field: "emoji", Message: "emoji is required"}
	}
	encoded := url.QueryEscape(emoji)
	if encoded == "" {
		return "", &types.ValidationError{Field: "emoji", Message: "invalid emoji"}
	}
	return encoded, nil
}
