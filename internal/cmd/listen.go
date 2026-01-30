package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	discordconfig "github.com/yourorg/arc-discord/gosdk/config"
	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	arcer "github.com/yourorg/arc-sdk/errors"
)

type interactionSubscriber interface {
	Subscribe(ctx context.Context, handler func(context.Context, []byte) error) error
	Close() error
}

type agentRegistryClient interface {
	Register(context.Context, AgentInfo) error
	Heartbeat(context.Context, AgentInfo, time.Duration) error
	Unregister(context.Context, string) error
	Close() error
}

type redisSubscriber struct {
	client    *redis.Client
	channel   string
	subscribe func(ctx context.Context, channel string) pubSub
}

type pubSub interface {
	Close() error
	ReceiveMessage(ctx context.Context) (*redis.Message, error)
}

var newRedisSubscriberFn = newRedisSubscriber

var newAgentRegistryFn = func(cfg redisConfig, ttl time.Duration) (agentRegistryClient, error) {
	return newAgentRegistry(cfg, ttl)
}

func newRedisSubscriber(cfg redisConfig, agent string) (interactionSubscriber, error) {
	channel := fmt.Sprintf("%s:agent:%s", normalizeChannelPrefix(cfg.ChannelPrefix), strings.ToLower(agent))
	client := redis.NewClient(newRedisOptions(cfg))
	ctx, cancel := context.WithTimeout(context.Background(), redisPublishTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return &redisSubscriber{
		client:  client,
		channel: channel,
		subscribe: func(ctx context.Context, channel string) pubSub {
			return client.Subscribe(ctx, channel)
		},
	}, nil
}

func (s *redisSubscriber) Subscribe(ctx context.Context, handler func(context.Context, []byte) error) error {
	sub := s.subscribe(ctx, s.channel)
	defer sub.Close()
	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, redis.ErrClosed) {
				return nil
			}
			return err
		}
		if handler != nil {
			if err := handler(ctx, []byte(msg.Payload)); err != nil {
				return err
			}
		}
	}
}

func (s *redisSubscriber) Close() error {
	return s.client.Close()
}

type outputPrinter interface {
	Printf(string, ...interface{})
}

type interactionResponder interface {
	EditOriginalInteractionResponse(ctx context.Context, applicationID, token string, params *types.MessageEditParams) (*types.Message, error)
	CreateFollowupMessage(ctx context.Context, applicationID, token string, params *types.MessageCreateParams) (*types.Message, error)
}

type agentListener struct {
	agentID       string
	applicationID string
	client        interactionResponder
	output        outputPrinter
}

func newAgentListener(agentID, appID string, cli interactionResponder, out outputPrinter) *agentListener {
	return &agentListener{
		agentID:       agentID,
		applicationID: appID,
		client:        cli,
		output:        out,
	}
}

func (l *agentListener) handlePayload(ctx context.Context, payload []byte) error {
	var env redisEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		l.output.Printf("invalid payload: %v\n", err)
		return nil
	}
	if strings.ToLower(env.Agent) != strings.ToLower(l.agentID) {
		return nil
	}
	var interaction types.Interaction
	if err := json.Unmarshal(env.Interaction, &interaction); err != nil {
		return fmt.Errorf("decode interaction: %w", err)
	}
	if interaction.Token == "" {
		return fmt.Errorf("interaction missing token")
	}
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	content := fmt.Sprintf("Agent %s received %s `%s` at %s", l.agentID, env.Kind, env.Key, time.Now().Format(time.RFC3339))
	params := &types.MessageEditParams{Content: content}
	if _, err := l.client.EditOriginalInteractionResponse(opCtx, l.applicationID, interaction.Token, params); err != nil {
		return fmt.Errorf("edit original response: %w", err)
	}
	followup := &types.MessageCreateParams{Content: fmt.Sprintf("Follow-up: %s completed %s `%s`", l.agentID, env.Kind, env.Key)}
	if _, err := l.client.CreateFollowupMessage(opCtx, l.applicationID, interaction.Token, followup); err != nil {
		return fmt.Errorf("create followup response: %w", err)
	}
	l.output.Printf("Processed %s interaction %s\n", env.Kind, env.Key)
	return nil
}

func agentCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Discord agent listeners",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(agentListenCmd(opts))
	return cmd
}

func agentListenCmd(opts *globalOptions) *cobra.Command {
	var (
		agentID     string
		redisAddr   string
		redisDB     int
		redisPass   string
		redisPrefix string
	)

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Subscribe to interaction events and respond via the Discord API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentListen(cmd, opts, agentListenOptions{
				AgentID:     agentID,
				RedisAddr:   redisAddr,
				RedisDB:     redisDB,
				RedisPass:   redisPass,
				RedisPrefix: redisPrefix,
			})
		},
		Example: `Example:
  VIBE_AGENT_ID=claude arc-discord agent listen

Example:
  VIBE_AGENT_ID=triage arc-discord agent listen --redis-addr redis://localhost:6379

Example:
  VIBE_AGENT_ID=reviewer arc-discord agent listen --redis-prefix arc:discord --redis-db 1`,
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent identifier (default $VIBE_AGENT_ID)")
	cmd.Flags().StringVar(&redisAddr, "redis-addr", "", "Redis address for subscriptions")
	cmd.Flags().IntVar(&redisDB, "redis-db", 0, "Redis database index")
	cmd.Flags().StringVar(&redisPass, "redis-password", "", "Redis password")
	cmd.Flags().StringVar(&redisPrefix, "redis-prefix", "", "Redis channel prefix (default arc:discord)")
	return cmd
}

type agentListenOptions struct {
	AgentID     string
	RedisAddr   string
	RedisDB     int
	RedisPass   string
	RedisPrefix string
}

func runAgentListen(cmd *cobra.Command, opts *globalOptions, overrides agentListenOptions) error {
	cfg, extra, _, err := opts.loadConfigWithInteractions()
	if err != nil {
		return err
	}

	agentID := overrides.AgentID
	if agentID == "" {
		agentID = os.Getenv(envDefaultAgentID)
	}
	if agentID == "" {
		return &arcer.CLIError{Msg: "agent id required", Hint: "set --agent or VIBE_AGENT_ID"}
	}
	if overrides.RedisAddr != "" {
		extra.Redis.Addr = overrides.RedisAddr
	}
	if overrides.RedisPass != "" {
		extra.Redis.Password = overrides.RedisPass
	}
	if overrides.RedisDB != 0 {
		extra.Redis.DB = overrides.RedisDB
	}
	if overrides.RedisPrefix != "" {
		extra.Redis.ChannelPrefix = overrides.RedisPrefix
	}
	extra.Redis.ChannelPrefix = normalizeChannelPrefix(extra.Redis.ChannelPrefix)
	if cfg.Discord.ApplicationID == "" {
		return &arcer.CLIError{Msg: "discord.application_id is required to edit responses"}
	}

	redisSub, err := newRedisSubscriberFn(extra.Redis, agentID)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to connect to redis"}).WithCause(err)
	}
	defer redisSub.Close()

	interactionClient, err := newInteractionClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize interaction client"}).WithCause(err)
	}

	registry, err := newAgentRegistryFn(extra.Redis, defaultRegistryTTL)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize agent registry"}).WithCause(err)
	}
	defer registry.Close()

	channelName := fmt.Sprintf("%s:agent:%s", normalizeChannelPrefix(extra.Redis.ChannelPrefix), strings.ToLower(agentID))
	info := agentInfo(agentID, extra.Interactions.Handlers, channelName)
	baseCtx := cmd.Context()
	if err := registry.Register(baseCtx, info); err != nil {
		return (&arcer.CLIError{Msg: "failed to register agent"}).WithCause(err)
	}

	hbCtx, hbCancel := context.WithCancel(baseCtx)
	defer hbCancel()
	go registry.Heartbeat(hbCtx, info, defaultHeartbeatInterval)
	defer registry.Unregister(context.Background(), agentID)

	listener := newAgentListener(agentID, cfg.Discord.ApplicationID, interactionClient, cmd)

	cmd.Printf("Listening for interactions as agent %s (channel prefix %s)\n", agentID, extra.Redis.ChannelPrefix)
	ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt)
	defer stop()

	err = redisSub.Subscribe(ctx, func(ctx context.Context, payload []byte) error {
		return listener.handlePayload(ctx, payload)
	})
	if err != nil {
		return (&arcer.CLIError{Msg: "listener exited with error"}).WithCause(err)
	}
	return nil
}

var newInteractionClientFn = createInteractionClient

func createInteractionClient(cfg *discordconfig.Config, token string) (interactionResponder, error) {
	rawClient, err := createRawDiscordClient(cfg, token)
	if err != nil {
		return nil, err
	}
	return interactions.NewInteractionClient(rawClient), nil
}

func createRawDiscordClient(cfg *discordconfig.Config, token string) (*client.Client, error) {
	if cfg == nil {
		cfg = discordconfig.Default()
	}
	if token == "" {
		token = cfg.Discord.BotToken
	}
	if token == "" {
		return nil, errors.New("no bot token configured")
	}
	opts := []client.Option{
		client.WithTimeout(cfg.Client.Timeout),
		client.WithMaxRetries(cfg.Client.Retries),
		client.WithStrategyName(cfg.Client.RateLimit.Strategy),
	}
	return client.New(token, opts...)
}
