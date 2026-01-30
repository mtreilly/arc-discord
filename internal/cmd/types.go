package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddr          = "127.0.0.1:8080"
	defaultRedisAddr           = "127.0.0.1:6379"
	defaultRedisPrefix         = "arc:discord"
	defaultInteractionTimeout  = 15 * time.Minute
	defaultHandlerEnabled      = true
	envDiscordPublicKey        = "VIBE_DISCORD_PUBLIC_KEY"
	envDiscordPublicURL        = "VIBE_DISCORD_PUBLIC_URL"
	envDefaultAgentID          = "VIBE_AGENT_ID"
	envDefaultRedisAddr        = "VIBE_DISCORD_REDIS_ADDR"
	envDefaultRedisPassword    = "VIBE_DISCORD_REDIS_PASSWORD"
	envDefaultRedisChannelPref = "VIBE_DISCORD_REDIS_PREFIX"
	envTunnelProvider          = "VIBE_DISCORD_TUNNEL_PROVIDER"
	envNgrokAuthToken          = "VIBE_DISCORD_NGROK_AUTH_TOKEN"
)

type interactionSettings struct {
	PublicKey    string
	PublicURL    string
	Server       serverConfig
	Redis        redisConfig
	Tunnel       tunnelConfig
	Interactions interactionsConfig
}

type serverConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type redisConfig struct {
	Addr          string `yaml:"addr"`
	DB            int    `yaml:"db"`
	Password      string `yaml:"password"`
	ChannelPrefix string `yaml:"channel_prefix"`
}

type tunnelConfig struct {
	Provider       string `yaml:"provider"`
	NgrokAuthToken string `yaml:"ngrok_auth_token"`
}

type interactionsConfig struct {
	Enabled  bool            `yaml:"enabled"`
	Timeout  time.Duration   `yaml:"timeout"`
	Handlers handlerMappings `yaml:"handlers"`
}

type handlerMappings struct {
	Commands     map[string]handlerRoute `yaml:"commands"`
	Components   map[string]handlerRoute `yaml:"components"`
	Modals       map[string]handlerRoute `yaml:"modals"`
	Autocomplete map[string]handlerRoute `yaml:"autocomplete"`
}

type handlerRoute struct {
	Agent       string               `yaml:"agent"`
	Channel     string               `yaml:"channel"`
	Description string               `yaml:"description"`
	Choices     []autocompleteChoice `yaml:"choices"`
}

type autocompleteChoice struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Value       interface{} `yaml:"value"`
}

type redisEnvelope struct {
	Agent          string          `json:"agent"`
	Kind           string          `json:"kind"`
	Key            string          `json:"key"`
	Interaction    json.RawMessage `json:"interaction"`
	ReceivedAt     time.Time       `json:"received_at"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	Source         string          `json:"source"`
}

func defaultInteractionSettings() *interactionSettings {
	cfg := &interactionSettings{
		PublicKey: strings.TrimSpace(os.Getenv(envDiscordPublicKey)),
		PublicURL: strings.TrimSpace(os.Getenv(envDiscordPublicURL)),
		Server: serverConfig{
			ListenAddr: defaultListenAddr,
		},
		Redis: redisConfig{
			Addr:          envOrDefault(envDefaultRedisAddr, defaultRedisAddr),
			DB:            0,
			Password:      os.Getenv(envDefaultRedisPassword),
			ChannelPrefix: envOrDefault(envDefaultRedisChannelPref, defaultRedisPrefix),
		},
		Tunnel: tunnelConfig{},
		Interactions: interactionsConfig{
			Enabled: defaultHandlerEnabled,
			Timeout: defaultInteractionTimeout,
			Handlers: handlerMappings{
				Commands:     map[string]handlerRoute{},
				Components:   map[string]handlerRoute{},
				Modals:       map[string]handlerRoute{},
				Autocomplete: map[string]handlerRoute{},
			},
		},
	}
	ensureHandlerMaps(&cfg.Interactions)
	if provider := strings.TrimSpace(os.Getenv(envTunnelProvider)); provider != "" {
		cfg.Tunnel.Provider = provider
	}
	if token := strings.TrimSpace(os.Getenv(envNgrokAuthToken)); token != "" {
		cfg.Tunnel.NgrokAuthToken = token
	}
	return cfg
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func ensureHandlerMaps(cfg *interactionsConfig) {
	if cfg == nil {
		return
	}
	if cfg.Handlers.Commands == nil {
		cfg.Handlers.Commands = make(map[string]handlerRoute)
	}
	if cfg.Handlers.Components == nil {
		cfg.Handlers.Components = make(map[string]handlerRoute)
	}
	if cfg.Handlers.Modals == nil {
		cfg.Handlers.Modals = make(map[string]handlerRoute)
	}
	if cfg.Handlers.Autocomplete == nil {
		cfg.Handlers.Autocomplete = make(map[string]handlerRoute)
	}
}

func normalizeChannelPrefix(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		return defaultRedisPrefix
	}
	return prefix
}
