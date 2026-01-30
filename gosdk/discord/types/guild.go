package types

import "time"

// Guild represents a Discord guild (server).
type Guild struct {
	ID                          string         `json:"id"`
	Name                        string         `json:"name"`
	Icon                        string         `json:"icon,omitempty"`
	IconHash                    string         `json:"icon_hash,omitempty"`
	Splash                      string         `json:"splash,omitempty"`
	DiscoverySplash             string         `json:"discovery_splash,omitempty"`
	Owner                       bool           `json:"owner,omitempty"`
	OwnerID                     string         `json:"owner_id"`
	Permissions                 string         `json:"permissions,omitempty"`
	Region                      string         `json:"region,omitempty"`
	AFKChannelID                string         `json:"afk_channel_id,omitempty"`
	AFKTimeout                  int            `json:"afk_timeout,omitempty"`
	WidgetEnabled               bool           `json:"widget_enabled,omitempty"`
	WidgetChannelID             string         `json:"widget_channel_id,omitempty"`
	VerificationLevel           int            `json:"verification_level,omitempty"`
	DefaultMessageNotifications int            `json:"default_message_notifications,omitempty"`
	ExplicitContentFilter       int            `json:"explicit_content_filter,omitempty"`
	Roles                       []Role         `json:"roles,omitempty"`
	Members                     []Member       `json:"members,omitempty"`
	Channels                    []Channel      `json:"channels,omitempty"`
	Description                 string         `json:"description,omitempty"`
	Banner                      string         `json:"banner,omitempty"`
	Features                    []string       `json:"features,omitempty"`
	ApproximateMemberCount      int            `json:"approximate_member_count,omitempty"`
	ApproximatePresenceCount    int            `json:"approximate_presence_count,omitempty"`
	WelcomeScreen               *WelcomeScreen `json:"welcome_screen,omitempty"`
}

// GuildModifyParams represents the payload for modifying a guild.
type GuildModifyParams struct {
	Name                        string `json:"name,omitempty"`
	Region                      string `json:"region,omitempty"`
	VerificationLevel           int    `json:"verification_level,omitempty"`
	DefaultMessageNotifications int    `json:"default_message_notifications,omitempty"`
	ExplicitContentFilter       int    `json:"explicit_content_filter,omitempty"`
	AFKChannelID                string `json:"afk_channel_id,omitempty"`
	AFKTimeout                  int    `json:"afk_timeout,omitempty"`
	Splash                      string `json:"splash,omitempty"`
	Banner                      string `json:"banner,omitempty"`
	Description                 string `json:"description,omitempty"`
	PreferredLocale             string `json:"preferred_locale,omitempty"`
	AuditLogReason              string `json:"-"`
}

// Role represents a guild role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	Position    int    `json:"position"`
	Color       int    `json:"color"`
	Hoist       bool   `json:"hoist"`
	Managed     bool   `json:"managed"`
	Mentionable bool   `json:"mentionable"`
}

// RoleCreateParams represents payload for creating a role.
type RoleCreateParams struct {
	Name        string `json:"name,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	Color       int    `json:"color,omitempty"`
	Hoist       bool   `json:"hoist,omitempty"`
	Mentionable bool   `json:"mentionable,omitempty"`
	AuditLogReason string `json:"-"`
}

// RoleModifyParams represents payload for updating a role.
type RoleModifyParams struct {
	Name        string `json:"name,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	Color       int    `json:"color,omitempty"`
	Hoist       bool   `json:"hoist,omitempty"`
	Mentionable bool   `json:"mentionable,omitempty"`
	AuditLogReason string `json:"-"`
}

// Member represents a guild member.
type Member struct {
	User         *User      `json:"user,omitempty"`
	Nick         string     `json:"nick,omitempty"`
	Roles        []string   `json:"roles"`
	JoinedAt     time.Time  `json:"joined_at"`
	PremiumSince *time.Time `json:"premium_since,omitempty"`
	Deaf         bool       `json:"deaf"`
	Mute         bool       `json:"mute"`
	Pending      bool       `json:"pending,omitempty"`
}

// ListMembersParams controls pagination when listing guild members.
type ListMembersParams struct {
	Limit int
	After string
}

// GuildPreview provides limited information about a guild.
type GuildPreview struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Icon                     string   `json:"icon,omitempty"`
	Splash                   string   `json:"splash,omitempty"`
	DiscoverySplash          string   `json:"discovery_splash,omitempty"`
	Emojis                   []Emoji  `json:"emojis,omitempty"`
	Features                 []string `json:"features,omitempty"`
	ApproximateMemberCount   int      `json:"approximate_member_count"`
	ApproximatePresenceCount int      `json:"approximate_presence_count"`
	Description              string   `json:"description,omitempty"`
}

// Emoji represents a custom emoji.
type Emoji struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Roles     []string `json:"roles,omitempty"`
	Available bool     `json:"available"`
}

// WelcomeScreen describes the welcome screen configuration.
type WelcomeScreen struct {
	Description     string                 `json:"description,omitempty"`
	WelcomeChannels []WelcomeScreenChannel `json:"welcome_channels"`
}

type WelcomeScreenChannel struct {
	ChannelID   string `json:"channel_id"`
	Description string `json:"description"`
	EmojiID     string `json:"emoji_id,omitempty"`
	EmojiName   string `json:"emoji_name,omitempty"`
}

// Validate ensures guild fields meet Discord requirements.
func (g *Guild) Validate() error {
	if g == nil {
		return nil
	}
	if g.ID == "" {
		return &ValidationError{Field: "id", Message: "guild ID is required"}
	}
	if g.Name == "" {
		return &ValidationError{Field: "name", Message: "guild name is required"}
	}
	return nil
}

// Validate ensures role fields are valid.
func (r *Role) Validate() error {
	if r == nil {
		return nil
	}
	if r.Name == "" {
		return &ValidationError{Field: "role.name", Message: "role name is required"}
	}
	return nil
}

// Validate ensures create params are valid.
func (p *RoleCreateParams) Validate() error {
	if p == nil {
		return &ValidationError{Field: "params", Message: "role create params required"}
	}
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "role name is required"}
	}
	return nil
}

// Validate ensures modify params are valid.
func (p *RoleModifyParams) Validate() error {
	if p == nil {
		return &ValidationError{Field: "params", Message: "role modify params required"}
	}
	return nil
}

// Validate ensures member list params are within Discord bounds.
func (p *ListMembersParams) Validate() error {
	if p == nil {
		return nil
	}
	if p.Limit < 0 || p.Limit > 1000 {
		return &ValidationError{Field: "limit", Message: "limit must be between 0 and 1000"}
	}
	return nil
}

// Validate ensures guild modification payloads are valid.
func (p *GuildModifyParams) Validate() error {
	if p == nil {
		return &ValidationError{Field: "params", Message: "guild modify params required"}
	}
	if p.Name != "" && len(p.Name) > 100 {
		return &ValidationError{Field: "name", Message: "guild name exceeds 100 characters"}
	}
	return nil
}
