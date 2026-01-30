package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRegistryTTL       = 2 * time.Minute
	defaultHeartbeatInterval = 30 * time.Second
	registryKeySuffix        = "registry"
)

type redisCommander interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

type agentRegistry struct {
	client redisCommander
	ttl    time.Duration
	prefix string
}

type AgentInfo struct {
	Agent        string    `json:"agent"`
	Capabilities []string  `json:"capabilities,omitempty"`
	Channels     []string  `json:"channels,omitempty"`
	Hostname     string    `json:"hostname,omitempty"`
	ProcessID    int       `json:"process_id,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func newAgentRegistry(cfg redisConfig, ttl time.Duration) (*agentRegistry, error) {
	if ttl <= 0 {
		ttl = defaultRegistryTTL
	}
	client := redis.NewClient(newRedisOptions(cfg))
	ctx, cancel := context.WithTimeout(context.Background(), redisPublishTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis registry: %w", err)
	}
	prefix := fmt.Sprintf("%s:%s", normalizeChannelPrefix(cfg.ChannelPrefix), registryKeySuffix)
	return &agentRegistry{
		client: client,
		ttl:    ttl,
		prefix: prefix,
	}, nil
}

func (r *agentRegistry) Register(ctx context.Context, info AgentInfo) error {
	if strings.TrimSpace(info.Agent) == "" {
		return fmt.Errorf("agent is required for registry entry")
	}
	info.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal agent info: %w", err)
	}
	if err := r.client.Set(ctx, r.key(info.Agent), payload, r.ttl).Err(); err != nil {
		return fmt.Errorf("store registry info: %w", err)
	}
	return nil
}

func (r *agentRegistry) Heartbeat(ctx context.Context, info AgentInfo, interval time.Duration) error {
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_ = r.Register(ctx, info)
		}
	}
}

func (r *agentRegistry) Unregister(ctx context.Context, agent string) error {
	if strings.TrimSpace(agent) == "" {
		return nil
	}
	if err := r.client.Del(ctx, r.key(agent)).Err(); err != nil {
		return fmt.Errorf("remove registry entry: %w", err)
	}
	return nil
}

func (r *agentRegistry) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *agentRegistry) key(agent string) string {
	return fmt.Sprintf("%s:%s", r.prefix, strings.ToLower(agent))
}

func agentInfo(agent string, handlers handlerMappings, channel string) AgentInfo {
	return AgentInfo{
		Agent:        agent,
		Capabilities: resolveAgentCapabilities(agent, handlers),
		Channels:     []string{channel},
		Hostname:     hostnameOrUnknown(),
		ProcessID:    os.Getpid(),
	}
}

func resolveAgentCapabilities(agent string, mappings handlerMappings) []string {
	var caps []string
	lowerAgent := strings.ToLower(agent)

	for name, route := range mappings.Commands {
		if strings.EqualFold(route.Agent, lowerAgent) {
			caps = append(caps, "command:"+name)
		}
	}
	for customID, route := range mappings.Components {
		if strings.EqualFold(route.Agent, lowerAgent) {
			caps = append(caps, "component:"+customID)
		}
	}
	for customID, route := range mappings.Modals {
		if strings.EqualFold(route.Agent, lowerAgent) {
			caps = append(caps, "modal:"+customID)
		}
	}
	for name, route := range mappings.Autocomplete {
		if strings.EqualFold(route.Agent, lowerAgent) {
			caps = append(caps, "autocomplete:"+name)
		}
	}
	sort.Strings(caps)
	return caps
}

func hostnameOrUnknown() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "unknown"
	}
	return host
}

func newAgentRegistryWithClient(client redisCommander, ttl time.Duration, prefix string) *agentRegistry {
	if ttl <= 0 {
		ttl = defaultRegistryTTL
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = fmt.Sprintf("%s:%s", defaultRedisPrefix, registryKeySuffix)
	}
	return &agentRegistry{
		client: client,
		ttl:    ttl,
		prefix: prefix,
	}
}
