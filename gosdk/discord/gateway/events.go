package gateway

import (
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Event defines a polymorphic gateway event payload.
type Event interface {
	Type() string
}

const (
	EventReady             = "READY"
	EventMessageCreate     = "MESSAGE_CREATE"
	EventMessageUpdate     = "MESSAGE_UPDATE"
	EventMessageDelete     = "MESSAGE_DELETE"
	EventGuildCreate       = "GUILD_CREATE"
	EventGuildUpdate       = "GUILD_UPDATE"
	EventGuildDelete       = "GUILD_DELETE"
	EventInteractionCreate = "INTERACTION_CREATE"
)

// ReadyEvent signals the gateway is ready for the client.
type ReadyEvent struct {
	V         int            `json:"v"`
	User      *types.User    `json:"user"`
	Guilds    []*types.Guild `json:"guilds"`
	SessionID string         `json:"session_id,omitempty"`
	ResumeURL string         `json:"resume_gateway_url,omitempty"`
}

func (e *ReadyEvent) Type() string { return EventReady }

// MessageCreateEvent fires when a new message is created.
type MessageCreateEvent struct {
	*types.Message
}

func (e *MessageCreateEvent) Type() string { return EventMessageCreate }

// MessageUpdateEvent fires when a message is updated.
type MessageUpdateEvent struct {
	*types.Message
}

func (e *MessageUpdateEvent) Type() string { return EventMessageUpdate }

// MessageDeleteEvent fires when a message is deleted.
type MessageDeleteEvent struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id,omitempty"`
}

func (e *MessageDeleteEvent) Type() string { return EventMessageDelete }

// InteractionCreateEvent signals component/message interaction data.
type InteractionCreateEvent struct {
	*types.Interaction
}

func (e *InteractionCreateEvent) Type() string { return EventInteractionCreate }

// GuildCreateEvent occurs when the client joins a guild.
type GuildCreateEvent struct {
	*types.Guild
}

func (e *GuildCreateEvent) Type() string { return EventGuildCreate }

// GuildUpdateEvent fires when guild metadata changes.
type GuildUpdateEvent struct {
	*types.Guild
}

func (e *GuildUpdateEvent) Type() string { return EventGuildUpdate }

// GuildDeleteEvent fires when the bot is removed from a guild.
type GuildDeleteEvent struct {
	GuildID     string `json:"id"`
	Unavailable bool   `json:"unavailable,omitempty"`
}

func (e *GuildDeleteEvent) Type() string { return EventGuildDelete }
