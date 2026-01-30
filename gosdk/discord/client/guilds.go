package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Guilds exposes guild-related REST helpers.
type Guilds struct {
	client *Client
}

// Guilds returns a guild service bound to the Client.
func (c *Client) Guilds() *Guilds {
	return &Guilds{client: c}
}

// GetGuild retrieves a guild by ID (optionally including approximate counts).
func (g *Guilds) GetGuild(ctx context.Context, guildID string, withCounts bool) (*types.Guild, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/guilds/%s", guildID)
	if withCounts {
		path += "?with_counts=true"
	}

	var guild types.Guild
	if err := g.client.Get(ctx, path, &guild); err != nil {
		return nil, err
	}
	return &guild, nil
}

// GetGuildPreview fetches a preview for a discoverable guild.
func (g *Guilds) GetGuildPreview(ctx context.Context, guildID string) (*types.GuildPreview, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}

	var preview types.GuildPreview
	if err := g.client.Get(ctx, fmt.Sprintf("/guilds/%s/preview", guildID), &preview); err != nil {
		return nil, err
	}
	return &preview, nil
}

// ModifyGuild updates mutable guild properties.
func (g *Guilds) ModifyGuild(ctx context.Context, guildID string, params *types.GuildModifyParams) (*types.Guild, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	headers := http.Header{}
	if params.AuditLogReason != "" {
		headers.Set("X-Audit-Log-Reason", url.QueryEscape(params.AuditLogReason))
	}

	var guild types.Guild
	if err := g.client.do(ctx, http.MethodPatch, fmt.Sprintf("/guilds/%s", guildID), params, &guild, headers); err != nil {
		return nil, err
	}
	return &guild, nil
}

// GetGuildChannels lists a guild's channels.
func (g *Guilds) GetGuildChannels(ctx context.Context, guildID string) ([]*types.Channel, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}

	var channels []*types.Channel
	if err := g.client.Get(ctx, fmt.Sprintf("/guilds/%s/channels", guildID), &channels); err != nil {
		return nil, err
	}
	return channels, nil
}

// CreateGuildChannel creates a channel within a guild.
func (g *Guilds) CreateGuildChannel(ctx context.Context, guildID string, params *types.ChannelCreateParams) (*types.Channel, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, &types.ValidationError{Field: "params", Message: "channel params required"}
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	headers := http.Header{}
	if params.AuditLogReason != "" {
		headers.Set("X-Audit-Log-Reason", url.QueryEscape(params.AuditLogReason))
	}

	var channel types.Channel
	if err := g.client.do(ctx, http.MethodPost, fmt.Sprintf("/guilds/%s/channels", guildID), params, &channel, headers); err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetGuildRoles retrieves roles in a guild.
func (g *Guilds) GetGuildRoles(ctx context.Context, guildID string) ([]*types.Role, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	var roles []*types.Role
	if err := g.client.Get(ctx, fmt.Sprintf("/guilds/%s/roles", guildID), &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// CreateGuildRole creates a role with optional audit log reason.
func (g *Guilds) CreateGuildRole(ctx context.Context, guildID string, params *types.RoleCreateParams) (*types.Role, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	headers := http.Header{}
	if params.AuditLogReason != "" {
		headers.Set("X-Audit-Log-Reason", url.QueryEscape(params.AuditLogReason))
	}
	var role types.Role
	if err := g.client.do(ctx, http.MethodPost, fmt.Sprintf("/guilds/%s/roles", guildID), params, &role, headers); err != nil {
		return nil, err
	}
	return &role, nil
}

// ModifyGuildRole updates a role.
func (g *Guilds) ModifyGuildRole(ctx context.Context, guildID, roleID string, params *types.RoleModifyParams) (*types.Role, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := validateID("roleID", roleID); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	headers := http.Header{}
	if params.AuditLogReason != "" {
		headers.Set("X-Audit-Log-Reason", url.QueryEscape(params.AuditLogReason))
	}
	var role types.Role
	if err := g.client.do(ctx, http.MethodPatch, fmt.Sprintf("/guilds/%s/roles/%s", guildID, roleID), params, &role, headers); err != nil {
		return nil, err
	}
	return &role, nil
}

// DeleteGuildRole removes a role from the guild.
func (g *Guilds) DeleteGuildRole(ctx context.Context, guildID, roleID string) error {
	if err := validateID("guildID", guildID); err != nil {
		return err
	}
	if err := validateID("roleID", roleID); err != nil {
		return err
	}
	return g.client.Delete(ctx, fmt.Sprintf("/guilds/%s/roles/%s", guildID, roleID))
}

// GetGuildMember retrieves a guild member.
func (g *Guilds) GetGuildMember(ctx context.Context, guildID, userID string) (*types.Member, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := validateID("userID", userID); err != nil {
		return nil, err
	}
	var member types.Member
	if err := g.client.Get(ctx, fmt.Sprintf("/guilds/%s/members/%s", guildID, userID), &member); err != nil {
		return nil, err
	}
	return &member, nil
}

// ListGuildMembers lists members with pagination.
func (g *Guilds) ListGuildMembers(ctx context.Context, guildID string, params *types.ListMembersParams) ([]*types.Member, error) {
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if params != nil {
		if err := params.Validate(); err != nil {
			return nil, err
		}
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
	path := fmt.Sprintf("/guilds/%s/members", guildID)
	if q := query.Encode(); q != "" {
		path += "?" + q
	}
	var members []*types.Member
	if err := g.client.Get(ctx, path, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// AddGuildMemberRole assigns a role to a member.
func (g *Guilds) AddGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	if err := validateID("guildID", guildID); err != nil {
		return err
	}
	if err := validateID("userID", userID); err != nil {
		return err
	}
	if err := validateID("roleID", roleID); err != nil {
		return err
	}
	return g.client.Put(ctx, fmt.Sprintf("/guilds/%s/members/%s/roles/%s", guildID, userID, roleID), nil, nil)
}

// RemoveGuildMemberRole removes a role from a member.
func (g *Guilds) RemoveGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	if err := validateID("guildID", guildID); err != nil {
		return err
	}
	if err := validateID("userID", userID); err != nil {
		return err
	}
	if err := validateID("roleID", roleID); err != nil {
		return err
	}
	return g.client.Delete(ctx, fmt.Sprintf("/guilds/%s/members/%s/roles/%s", guildID, userID, roleID))
}
