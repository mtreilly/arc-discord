package cmd

import (
	"context"

	discordconfig "github.com/yourorg/arc-discord/gosdk/config"
	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/discord/webhook"
)

type webhookDispatcher interface {
	Send(ctx context.Context, msg *types.WebhookMessage) error
	SendWithFiles(ctx context.Context, msg *types.WebhookMessage, files []webhook.FileAttachment) error
	CreateThread(ctx context.Context, threadName string, msg *types.WebhookMessage) error
}

type botClient interface {
	Messages() messageService
	Channels() channelService
	Guilds() guildService
	ApplicationCommands(applicationID string) applicationCommandService
}

type messageService interface {
	CreateMessage(ctx context.Context, channelID string, params *types.MessageCreateParams) (*types.Message, error)
	EditMessage(ctx context.Context, channelID, messageID string, params *types.MessageEditParams) (*types.Message, error)
	DeleteMessage(ctx context.Context, channelID, messageID string) error
	CreateReaction(ctx context.Context, channelID, messageID, emoji string) error
	DeleteOwnReaction(ctx context.Context, channelID, messageID, emoji string) error
}

type channelService interface {
	GetChannel(ctx context.Context, channelID string) (*types.Channel, error)
	GetChannelMessages(ctx context.Context, channelID string, params *client.GetChannelMessagesParams) ([]*types.Message, error)
	ModifyChannel(ctx context.Context, channelID string, params *types.ModifyChannelParams) (*types.Channel, error)
}

type guildService interface {
	GetGuild(ctx context.Context, guildID string, withCounts bool) (*types.Guild, error)
	ListGuildMembers(ctx context.Context, guildID string, params *types.ListMembersParams) ([]*types.Member, error)
	GetGuildRoles(ctx context.Context, guildID string) ([]*types.Role, error)
	GetGuildChannels(ctx context.Context, guildID string) ([]*types.Channel, error)
}

type applicationCommandService interface {
	GetGlobalApplicationCommands(ctx context.Context) ([]*types.ApplicationCommand, error)
	GetGuildApplicationCommands(ctx context.Context, guildID string) ([]*types.ApplicationCommand, error)
	CreateGlobalApplicationCommand(ctx context.Context, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error)
	CreateGuildApplicationCommand(ctx context.Context, guildID string, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error)
	DeleteGlobalApplicationCommand(ctx context.Context, commandID string) error
	DeleteGuildApplicationCommand(ctx context.Context, guildID, commandID string) error
}

type realBotClient struct {
	inner *client.Client
}

func (r *realBotClient) Messages() messageService {
	return r.inner.Messages()
}

func (r *realBotClient) Channels() channelService {
	return r.inner.Channels()
}

func (r *realBotClient) Guilds() guildService {
	return r.inner.Guilds()
}

func (r *realBotClient) ApplicationCommands(applicationID string) applicationCommandService {
	return r.inner.ApplicationCommands(applicationID)
}

func createWebhookClient(cfg *discordconfig.Config, webhookURL string) (webhookDispatcher, error) {
	if cfg == nil {
		cfg = discordconfig.Default()
	}
	opts := []webhook.Option{
		webhook.WithTimeout(cfg.Client.Timeout),
		webhook.WithMaxRetries(cfg.Client.Retries),
		webhook.WithStrategyName(cfg.Client.RateLimit.Strategy),
	}
	return webhook.NewClient(webhookURL, opts...)
}

func createBotClient(cfg *discordconfig.Config, token string) (botClient, error) {
	raw, err := createRawDiscordClient(cfg, token)
	if err != nil {
		return nil, err
	}
	return &realBotClient{inner: raw}, nil
}
