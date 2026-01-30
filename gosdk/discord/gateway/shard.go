package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/yourorg/arc-discord/gosdk/logger"
)

const defaultGatewayBotURL = "https://discord.com/api/v10/gateway/bot"

// Shard represents a gateway shard (ID + total + client).
type Shard struct {
	id          int
	totalShards int
	client      *Client
}

// ShardManagerOption configures the shard manager.
type ShardManagerOption func(*ShardManager)

// WithShardLogger overrides the logger.
func WithShardLogger(l *logger.Logger) ShardManagerOption {
	return func(sm *ShardManager) {
		if l != nil {
			sm.logger = l
		}
	}
}

// WithShardDispatcher uses a custom dispatcher.
func WithShardDispatcher(d *Dispatcher) ShardManagerOption {
	return func(sm *ShardManager) {
		if d != nil {
			sm.dispatcher = d
		}
	}
}

// WithShardConnectionOptions augments the underlying connections.
func WithShardConnectionOptions(opts ...ConnectionOption) ShardManagerOption {
	return func(sm *ShardManager) {
		sm.connectionOpts = append(sm.connectionOpts, opts...)
	}
}

// WithShardGatewayBotURL overrides the /gateway/bot endpoint.
func WithShardGatewayBotURL(url string) ShardManagerOption {
	return func(sm *ShardManager) {
		if url != "" {
			sm.gatewayBotURL = url
		}
	}
}

// WithShardGatewayHTTPClient overrides the HTTP client used for gateway/bot.
func WithShardGatewayHTTPClient(client *http.Client) ShardManagerOption {
	return func(sm *ShardManager) {
		if client != nil {
			sm.gatewayBotClient = client
		}
	}
}

// ShardManager orchestrates multiple gateway shards.
type ShardManager struct {
	token            string
	intents          int
	shardCount       int
	logger           *logger.Logger
	dispatcher       *Dispatcher
	connectionOpts   []ConnectionOption
	gatewayBotURL    string
	gatewayBotClient *http.Client

	shards []*Shard
	mu     sync.Mutex
}

// NewShardManager constructs a shard manager.
func NewShardManager(token string, shardCount int, intents int, opts ...ShardManagerOption) *ShardManager {
	sm := &ShardManager{
		token:            token,
		intents:          intents,
		shardCount:       shardCount,
		logger:           logger.Default(),
		dispatcher:       NewDispatcher(),
		gatewayBotURL:    defaultGatewayBotURL,
		gatewayBotClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(sm)
	}
	return sm
}

// Connect initializes and starts all shard clients.
func (sm *ShardManager) Connect(ctx context.Context) error {
	sm.mu.Lock()
	if len(sm.shards) > 0 {
		sm.mu.Unlock()
		return errors.New("shard manager already connected")
	}
	sm.mu.Unlock()

	for id := 0; id < sm.shardCount; id++ {
		connOpts := append([]ConnectionOption{}, sm.connectionOpts...)
		shardURL := fmt.Sprintf("%s&shard=%d,%d", defaultGatewayURL, id, sm.shardCount)
		connOpts = append(connOpts, WithGatewayURL(shardURL))

		client, err := NewClient(sm.token, sm.intents,
			WithDispatcher(sm.dispatcher),
			WithGatewayLogger(sm.logger),
			WithConnectionOptions(connOpts...),
		)
		if err != nil {
			return fmt.Errorf("init shard %d: %w", id, err)
		}
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("connect shard %d: %w", id, err)
		}

		sm.mu.Lock()
		sm.shards = append(sm.shards, &Shard{id: id, totalShards: sm.shardCount, client: client})
		sm.mu.Unlock()
	}
	return nil
}

// Disconnect closes all shard clients.
func (sm *ShardManager) Disconnect() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var errs []error
	for _, shard := range sm.shards {
		if err := shard.client.Disconnect(); err != nil {
			errs = append(errs, fmt.Errorf("shard %d: %w", shard.id, err))
		}
	}
	sm.shards = nil
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// On registers an event handler across all shards.
func (sm *ShardManager) On(eventType string, handler EventHandler) {
	sm.dispatcher.On(eventType, handler)
}

// OnMessageCreate registers a MESSAGE_CREATE handler.
func (sm *ShardManager) OnMessageCreate(handler func(context.Context, *MessageCreateEvent) error) {
	sm.dispatcher.OnMessageCreate(handler)
}

// OnMessageUpdate registers a MESSAGE_UPDATE handler.
func (sm *ShardManager) OnMessageUpdate(handler func(context.Context, *MessageUpdateEvent) error) {
	sm.dispatcher.OnMessageUpdate(handler)
}

// OnInteraction registers an INTERACTION_CREATE handler.
func (sm *ShardManager) OnInteraction(handler func(context.Context, *InteractionCreateEvent) error) {
	sm.dispatcher.OnInteraction(handler)
}

// Broadcast sends the payload to every shard.
func (sm *ShardManager) Broadcast(ctx context.Context, payload *Payload) error {
	sm.mu.Lock()
	shards := append([]*Shard(nil), sm.shards...)
	sm.mu.Unlock()
	var errs []error
	for _, shard := range shards {
		if err := shard.client.Send(ctx, payload); err != nil {
			errs = append(errs, fmt.Errorf("shard %d: %w", shard.id, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// AutoScale consults /gateway/bot and adjusts the shard count based on the provided strategy.
func (sm *ShardManager) AutoScale(ctx context.Context, guildCount int, strategy ShardingStrategy) error {
	if sm.token == "" {
		return errors.New("token required for autoscaling")
	}
	if strategy == nil {
		strategy = &RecommendedSharding{}
	}

	info, err := fetchGatewayBotInfo(ctx, sm.gatewayBotClient, sm.gatewayBotURL, sm.token)
	if err != nil {
		return fmt.Errorf("fetch gateway bot info: %w", err)
	}
	if setter, ok := strategy.(interface{ SetRecommended(int) }); ok {
		setter.SetRecommended(info.Shards)
	}

	count := strategy.Calculate(guildCount)
	if count <= 0 {
		count = info.Shards
	}
	if count <= 0 {
		count = 1
	}
	sm.mu.Lock()
	sm.shardCount = count
	sm.mu.Unlock()
	return nil
}

// ShardingStrategy calculates shard counts.
type ShardingStrategy interface {
	Calculate(guildCount int) int
}

// RecommendedSharding uses Discord's recommended shard count (or guild density) to choose a value.
type RecommendedSharding struct {
	recommended int
}

// SetRecommended updates the recommended shard count sourced from Discord.
func (r *RecommendedSharding) SetRecommended(count int) {
	r.recommended = count
}

// Calculate returns the configured recommended shards or a density-based estimate.
func (r *RecommendedSharding) Calculate(guildCount int) int {
	if r.recommended > 0 {
		return r.recommended
	}
	if guildCount <= 0 {
		return 1
	}
	const perShard = 2000
	return (guildCount / perShard) + 1
}

// FixedSharding always returns the configured count.
type FixedSharding struct {
	Count int
}

// Calculate returns the fixed shard count (minimum 1).
func (f FixedSharding) Calculate(_ int) int {
	if f.Count <= 0 {
		return 1
	}
	return f.Count
}

// GatewayBotInfo describes /gateway/bot responses.
type GatewayBotInfo struct {
	URL               string `json:"url"`
	Shards            int    `json:"shards"`
	SessionStartLimit struct {
		Total      int `json:"total"`
		Remaining  int `json:"remaining"`
		ResetAfter int `json:"reset_after"`
	} `json:"session_start_limit"`
}

func fetchGatewayBotInfo(ctx context.Context, client *http.Client, endpoint, token string) (*GatewayBotInfo, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("User-Agent", "agent-discord-gateway/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var info GatewayBotInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
