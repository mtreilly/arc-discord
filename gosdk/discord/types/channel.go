package types

import (
	"regexp"
	"time"
)

// ChannelType defines the Discord channel type.
type ChannelType int

const (
	ChannelTypeGuildText ChannelType = iota
	ChannelTypeDM
	ChannelTypeGuildVoice
	ChannelTypeGroupDM
	ChannelTypeGuildCategory
	ChannelTypeGuildNews
	ChannelTypeGuildStore
	ChannelTypeGuildNewsThread
	ChannelTypeGuildPublicThread
	ChannelTypeGuildPrivateThread
	ChannelTypeGuildStageVoice
	ChannelTypeGuildDirectory
	ChannelTypeGuildForum
)

// PermissionOverwriteType distinguishes role vs member overwrites.
type PermissionOverwriteType string

const (
	PermissionOverwriteRole   PermissionOverwriteType = "role"
	PermissionOverwriteMember PermissionOverwriteType = "member"
)

// ChannelFlags represents channel-level feature flags (bitmask).
type ChannelFlags uint64

const (
	ChannelFlagPinned                   ChannelFlags = 1 << 1
	ChannelFlagRequireTag               ChannelFlags = 1 << 4
	ChannelFlagHideMediaDownloadOptions ChannelFlags = 1 << 15
)

// PermissionOverwrite mirrors Discord's overwrite object.
type PermissionOverwrite struct {
	ID    string                  `json:"id"`
	Type  PermissionOverwriteType `json:"type"`
	Allow string                  `json:"allow"`
	Deny  string                  `json:"deny"`
}

// ThreadMetadata describes thread configuration (forum/text threads).
type ThreadMetadata struct {
	Archived            bool       `json:"archived"`
	AutoArchiveDuration int        `json:"auto_archive_duration"`
	ArchiveTimestamp    *time.Time `json:"archive_timestamp,omitempty"`
	Locked              bool       `json:"locked"`
	Invitable           bool       `json:"invitable,omitempty"`
	CreateTimestamp     *time.Time `json:"create_timestamp,omitempty"`
}

// Channel is the primary representation of Discord channel objects.
type Channel struct {
	ID                   string                `json:"id"`
	Type                 ChannelType           `json:"type"`
	GuildID              string                `json:"guild_id,omitempty"`
	Position             int                   `json:"position,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
	Name                 string                `json:"name,omitempty"`
	Topic                string                `json:"topic,omitempty"`
	NSFW                 bool                  `json:"nsfw,omitempty"`
	LastMessageID        string                `json:"last_message_id,omitempty"`
	Bitrate              int                   `json:"bitrate,omitempty"`
	UserLimit            int                   `json:"user_limit,omitempty"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user,omitempty"`
	Recipients           []User                `json:"recipients,omitempty"`
	Icon                 string                `json:"icon,omitempty"`
	OwnerID              string                `json:"owner_id,omitempty"`
	ApplicationID        string                `json:"application_id,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	LastPinTimestamp     *time.Time            `json:"last_pin_timestamp,omitempty"`
	RTCRegion            string                `json:"rtc_region,omitempty"`
	VideoQualityMode     int                   `json:"video_quality_mode,omitempty"`
	Flags                ChannelFlags          `json:"flags,omitempty"`
	ThreadMetadata       *ThreadMetadata       `json:"thread_metadata,omitempty"`
	Permissions          string                `json:"permissions,omitempty"`
	AvailableTags        []ForumTag            `json:"available_tags,omitempty"`
	DefaultReaction      *DefaultReaction      `json:"default_reaction_emoji,omitempty"`
	DefaultSortOrder     string                `json:"default_sort_order,omitempty"`
	DefaultForumLayout   string                `json:"default_forum_layout,omitempty"`
}

// ForumTag represents tags available for forum channels.
type ForumTag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Moderated bool   `json:"moderated"`
	EmojiID   string `json:"emoji_id,omitempty"`
	EmojiName string `json:"emoji_name,omitempty"`
}

// DefaultReaction describes default reaction emoji for forum threads.
type DefaultReaction struct {
	EmojiID   string `json:"emoji_id,omitempty"`
	EmojiName string `json:"emoji_name,omitempty"`
}

// ChannelCreateParams describes channel creation payloads.
type ChannelCreateParams struct {
	Name                 string                `json:"name,omitempty"`
	Type                 ChannelType           `json:"type,omitempty"`
	Topic                string                `json:"topic,omitempty"`
	Bitrate              int                   `json:"bitrate,omitempty"`
	UserLimit            int                   `json:"user_limit,omitempty"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user,omitempty"`
	Position             int                   `json:"position,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	NSFW                 bool                  `json:"nsfw,omitempty"`
	RTCRegion            string                `json:"rtc_region,omitempty"`
	VideoQualityMode     int                   `json:"video_quality_mode,omitempty"`
	DefaultAutoArchive   int                   `json:"default_auto_archive_duration,omitempty"`
	AvailableTags        []ForumTag            `json:"available_tags,omitempty"`
	DefaultReaction      *DefaultReaction      `json:"default_reaction_emoji,omitempty"`
	DefaultSortOrder     string                `json:"default_sort_order,omitempty"`
	AuditLogReason       string                `json:"-"`
}

// ModifyChannelParams mirrors update payloads (name optional).
type ModifyChannelParams struct {
	Name                 string                `json:"name,omitempty"`
	Type                 ChannelType           `json:"type,omitempty"`
	Topic                string                `json:"topic,omitempty"`
	Bitrate              int                   `json:"bitrate,omitempty"`
	UserLimit            int                   `json:"user_limit,omitempty"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user,omitempty"`
	Position             int                   `json:"position,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	NSFW                 bool                  `json:"nsfw,omitempty"`
	RTCRegion            string                `json:"rtc_region,omitempty"`
	VideoQualityMode     int                   `json:"video_quality_mode,omitempty"`
	AvailableTags        []ForumTag            `json:"available_tags,omitempty"`
	DefaultReaction      *DefaultReaction      `json:"default_reaction_emoji,omitempty"`
	DefaultSortOrder     string                `json:"default_sort_order,omitempty"`
	AuditLogReason       string                `json:"-"`
}

// Validate ensures Channel fields meet Discord constraints.
func (c *Channel) Validate() error {
	if c == nil {
		return nil
	}
	if err := validateChannelName(c.Name); err != nil {
		return err
	}
	if len(c.Topic) > 1024 {
		return &ValidationError{Field: "topic", Message: "topic exceeds 1024 characters"}
	}
	return nil
}

// Validate ensures creation params satisfy Discord requirements.
func (p *ChannelCreateParams) Validate() error {
	if p == nil {
		return &ValidationError{Field: "params", Message: "channel params required"}
	}
	if err := validateChannelName(p.Name); err != nil {
		return err
	}

	if len(p.Topic) > 1024 {
		return &ValidationError{Field: "topic", Message: "topic exceeds 1024 characters"}
	}

	if p.Bitrate < 0 {
		return &ValidationError{Field: "bitrate", Message: "bitrate cannot be negative"}
	}

	if p.UserLimit < 0 {
		return &ValidationError{Field: "user_limit", Message: "user limit cannot be negative"}
	}

	if p.RateLimitPerUser < 0 || p.RateLimitPerUser > 21600 {
		return &ValidationError{Field: "rate_limit_per_user", Message: "rate limit must be between 0 and 21600 seconds"}
	}

	return nil
}

// Validate ensures modify params satisfy Discord requirements.
func (p *ModifyChannelParams) Validate() error {
	if p == nil {
		return &ValidationError{Field: "params", Message: "modify params required"}
	}
	if p.Name != "" {
		if err := validateChannelName(p.Name); err != nil {
			return err
		}
	}
	if len(p.Topic) > 1024 {
		return &ValidationError{Field: "topic", Message: "topic exceeds 1024 characters"}
	}
	if p.Bitrate < 0 {
		return &ValidationError{Field: "bitrate", Message: "bitrate cannot be negative"}
	}
	if p.UserLimit < 0 {
		return &ValidationError{Field: "user_limit", Message: "user limit cannot be negative"}
	}
	if p.RateLimitPerUser < 0 || p.RateLimitPerUser > 21600 {
		return &ValidationError{Field: "rate_limit_per_user", Message: "rate limit must be between 0 and 21600 seconds"}
	}
	return nil
}

// ChannelParamsBuilder offers a fluent builder for ChannelCreateParams.
type ChannelParamsBuilder struct {
	params *ChannelCreateParams
}

// NewChannelParamsBuilder instantiates a builder with the required name/type.
func NewChannelParamsBuilder(name string, channelType ChannelType) *ChannelParamsBuilder {
	return &ChannelParamsBuilder{
		params: &ChannelCreateParams{
			Name: name,
			Type: channelType,
		},
	}
}

func (b *ChannelParamsBuilder) Topic(topic string) *ChannelParamsBuilder {
	b.params.Topic = topic
	return b
}

func (b *ChannelParamsBuilder) Parent(parentID string) *ChannelParamsBuilder {
	b.params.ParentID = parentID
	return b
}

func (b *ChannelParamsBuilder) NSFW(nsfw bool) *ChannelParamsBuilder {
	b.params.NSFW = nsfw
	return b
}

func (b *ChannelParamsBuilder) Bitrate(bitrate int) *ChannelParamsBuilder {
	b.params.Bitrate = bitrate
	return b
}

func (b *ChannelParamsBuilder) Build() (*ChannelCreateParams, error) {
	if err := b.params.Validate(); err != nil {
		return nil, err
	}
	return b.params, nil
}

var channelNamePattern = regexp.MustCompile(`^[\w\- ]{1,100}$`)

func validateChannelName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: "channel name is required"}
	}
	if len(name) > 100 {
		return &ValidationError{Field: "name", Message: "channel name exceeds 100 characters"}
	}
	if !channelNamePattern.MatchString(name) {
		return &ValidationError{Field: "name", Message: "channel name has invalid characters"}
	}
	return nil
}
