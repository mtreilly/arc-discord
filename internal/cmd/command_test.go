package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/yourorg/arc-sdk/output"
	discordconfig "github.com/yourorg/arc-discord/gosdk/config"
	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/discord/webhook"
)

func TestWebhookSendUsesDispatcher(t *testing.T) {
	cfg := testConfig()
	fake := &fakeWebhookClient{}
	hookStubs(t, cfg, fake, nil)

	opts := &globalOptions{output: output.OutputOptions{Format: string(output.OutputJSON)}}
	cmd := webhookSendCmd(opts)
	cmd.SetArgs([]string{"hello"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(fake.messages) != 1 {
		t.Fatalf("expected 1 webhook message, got %d", len(fake.messages))
	}
	if fake.messages[0].Content != "hello" {
		t.Fatalf("content mismatch: %s", fake.messages[0].Content)
	}
}

func TestMessageSendInvokesBot(t *testing.T) {
	cfg := testConfig()
	messageSvc := &fakeMessageService{}
	bot := &fakeBotClient{messageSvc: messageSvc, channelSvc: &fakeChannelService{}, guildSvc: &fakeGuildService{}}
	hookBot(t, cfg, bot)

	opts := &globalOptions{output: output.OutputOptions{Format: string(output.OutputJSON)}}
	cmd := messageSendCmd(opts)
	cmd.SetArgs([]string{"--channel", "123", "--content", "hi"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if messageSvc.channelID != "123" {
		t.Fatalf("channel expected 123, got %s", messageSvc.channelID)
	}
	if messageSvc.params == nil || messageSvc.params.Content != "hi" {
		t.Fatalf("params not captured: %#v", messageSvc.params)
	}
}

func TestChannelGet(t *testing.T) {
	cfg := testConfig()
	channelSvc := &fakeChannelService{channel: &types.Channel{ID: "42", Name: "alerts"}}
	bot := &fakeBotClient{messageSvc: &fakeMessageService{}, channelSvc: channelSvc, guildSvc: &fakeGuildService{}}
	hookBot(t, cfg, bot)

	opts := &globalOptions{output: output.OutputOptions{Format: string(output.OutputJSON)}}
	cmd := channelGetCmd(opts)
	cmd.SetArgs([]string{"--channel", "42"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if channelSvc.requested != "42" {
		t.Fatalf("expected channel lookup for 42, got %s", channelSvc.requested)
	}
}

func TestGuildGet(t *testing.T) {
	cfg := testConfig()
	guildSvc := &fakeGuildService{guild: &types.Guild{ID: "99", Name: "labs"}}
	bot := &fakeBotClient{messageSvc: &fakeMessageService{}, channelSvc: &fakeChannelService{}, guildSvc: guildSvc}
	hookBot(t, cfg, bot)

	opts := &globalOptions{output: output.OutputOptions{Format: string(output.OutputJSON)}}
	cmd := guildGetCmd(opts)
	cmd.SetArgs([]string{"--guild", "99"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if guildSvc.requested != "99" {
		t.Fatalf("expected guild lookup for 99, got %s", guildSvc.requested)
	}
}

func TestConfigShow(t *testing.T) {
	cfg := testConfig()
	hookStubs(t, cfg, &fakeWebhookClient{}, &fakeBotClient{})

	opts := &globalOptions{output: output.OutputOptions{Format: string(output.OutputJSON)}}
	cmd := configShowCmd(opts)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected output")
	}
}

type fakeWebhookClient struct {
	messages []*types.WebhookMessage
}

func (f *fakeWebhookClient) Send(_ context.Context, msg *types.WebhookMessage) error {
	f.messages = append(f.messages, msg)
	return nil
}

func (f *fakeWebhookClient) SendWithFiles(_ context.Context, msg *types.WebhookMessage, _ []webhook.FileAttachment) error {
	f.messages = append(f.messages, msg)
	return nil
}

func (f *fakeWebhookClient) CreateThread(_ context.Context, name string, msg *types.WebhookMessage) error {
	f.messages = append(f.messages, msg)
	return nil
}

type fakeBotClient struct {
	messageSvc *fakeMessageService
	channelSvc *fakeChannelService
	guildSvc   *fakeGuildService
	commandSvc *fakeApplicationCommands
}

func (f *fakeBotClient) Messages() messageService {
	return f.messageSvc
}

func (f *fakeBotClient) Channels() channelService {
	return f.channelSvc
}

func (f *fakeBotClient) Guilds() guildService {
	return f.guildSvc
}

func (f *fakeBotClient) ApplicationCommands(applicationID string) applicationCommandService {
	if f.commandSvc != nil {
		return f.commandSvc
	}
	return &fakeApplicationCommands{}
}

type fakeMessageService struct {
	channelID string
	params    *types.MessageCreateParams
}

func (f *fakeMessageService) CreateMessage(_ context.Context, channelID string, params *types.MessageCreateParams) (*types.Message, error) {
	f.channelID = channelID
	f.params = params
	return &types.Message{ID: "m1", ChannelID: channelID, Timestamp: time.Now()}, nil
}

func (f *fakeMessageService) EditMessage(_ context.Context, channelID, messageID string, params *types.MessageEditParams) (*types.Message, error) {
	return &types.Message{ID: messageID, ChannelID: channelID, Timestamp: time.Now()}, nil
}

func (f *fakeMessageService) DeleteMessage(_ context.Context, channelID, messageID string) error {
	return nil
}

func (f *fakeMessageService) CreateReaction(_ context.Context, channelID, messageID, emoji string) error {
	return nil
}

func (f *fakeMessageService) DeleteOwnReaction(_ context.Context, channelID, messageID, emoji string) error {
	return nil
}

type fakeChannelService struct {
	channel   *types.Channel
	requested string
}

func (f *fakeChannelService) GetChannel(_ context.Context, id string) (*types.Channel, error) {
	f.requested = id
	if f.channel != nil {
		return f.channel, nil
	}
	return &types.Channel{ID: id}, nil
}

func (f *fakeChannelService) GetChannelMessages(_ context.Context, channelID string, params *client.GetChannelMessagesParams) ([]*types.Message, error) {
	return []*types.Message{}, nil
}

func (f *fakeChannelService) ModifyChannel(_ context.Context, channelID string, params *types.ModifyChannelParams) (*types.Channel, error) {
	return &types.Channel{ID: channelID}, nil
}

type fakeGuildService struct {
	guild     *types.Guild
	requested string
}

func (f *fakeGuildService) GetGuild(_ context.Context, id string, _ bool) (*types.Guild, error) {
	f.requested = id
	if f.guild != nil {
		return f.guild, nil
	}
	return &types.Guild{ID: id}, nil
}

func (f *fakeGuildService) ListGuildMembers(_ context.Context, guildID string, params *types.ListMembersParams) ([]*types.Member, error) {
	return []*types.Member{}, nil
}

func (f *fakeGuildService) GetGuildRoles(_ context.Context, guildID string) ([]*types.Role, error) {
	return []*types.Role{}, nil
}

func (f *fakeGuildService) GetGuildChannels(_ context.Context, guildID string) ([]*types.Channel, error) {
	return []*types.Channel{}, nil
}

type fakeApplicationCommands struct{}

func (f *fakeApplicationCommands) GetGlobalApplicationCommands(ctx context.Context) ([]*types.ApplicationCommand, error) {
	return []*types.ApplicationCommand{}, nil
}

func (f *fakeApplicationCommands) GetGuildApplicationCommands(ctx context.Context, guildID string) ([]*types.ApplicationCommand, error) {
	return []*types.ApplicationCommand{}, nil
}

func (f *fakeApplicationCommands) CreateGlobalApplicationCommand(ctx context.Context, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	return cmd, nil
}

func (f *fakeApplicationCommands) CreateGuildApplicationCommand(ctx context.Context, guildID string, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	return cmd, nil
}

func (f *fakeApplicationCommands) DeleteGlobalApplicationCommand(ctx context.Context, commandID string) error {
	return nil
}

func (f *fakeApplicationCommands) DeleteGuildApplicationCommand(ctx context.Context, guildID, commandID string) error {
	return nil
}

func testConfig() *discordconfig.Config {
	cfg := discordconfig.Default()
	cfg.Discord.BotToken = "test-token"
	cfg.Discord.Webhooks = map[string]string{"default": "https://example.com/webhook"}
	return cfg
}

func hookStubs(t *testing.T, cfg *discordconfig.Config, webhookClient webhookDispatcher, bot botClient) {
	loadDiscordConfigFn = func(string) (*discordconfig.Config, string, error) {
		return cfg, "config.yaml", nil
	}
	newWebhookClientFn = func(*discordconfig.Config, string) (webhookDispatcher, error) {
		if webhookClient != nil {
			return webhookClient, nil
		}
		return &fakeWebhookClient{}, nil
	}
	newBotClientFn = func(*discordconfig.Config, string) (botClient, error) {
		if bot != nil {
			return bot, nil
		}
		return &fakeBotClient{messageSvc: &fakeMessageService{}, channelSvc: &fakeChannelService{}, guildSvc: &fakeGuildService{}}, nil
	}

	t.Cleanup(func() {
		loadDiscordConfigFn = loadDiscordConfig
		newWebhookClientFn = createWebhookClient
		newBotClientFn = createBotClient
	})
}

func hookBot(t *testing.T, cfg *discordconfig.Config, bot botClient) {
	hookStubs(t, cfg, &fakeWebhookClient{}, bot)
}
