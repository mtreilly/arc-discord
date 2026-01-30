package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourorg/arc-sdk/output"
	"github.com/yourorg/arc-sdk/utils"
	discordconfig "github.com/yourorg/arc-discord/gosdk/config"
	"gopkg.in/yaml.v3"
)

type globalOptions struct {
	configPath      string
	output          output.OutputOptions
	tokenOverride   string
	webhookOverride string
	profile         string
	environment     string
	rateStrategy    string
	appliedProfile  string
	appliedEnv      string
}

var (
	loadDiscordConfigFn = loadDiscordConfig
	newWebhookClientFn  = createWebhookClient
	newBotClientFn      = createBotClient
)

func (o *globalOptions) loadConfig() (*discordconfig.Config, string, error) {
	cfg, path, err := loadDiscordConfigFn(o.configPath)
	if err != nil {
		return nil, path, err
	}
	if err := o.applyProfile(cfg); err != nil {
		return nil, path, err
	}
	if err := o.applyEnvironment(cfg); err != nil {
		return nil, path, err
	}
	if o.rateStrategy != "" {
		cfg.Client.RateLimit.Strategy = o.rateStrategy
	}
	if o.tokenOverride != "" {
		cfg.Discord.BotToken = o.tokenOverride
	}
	if o.webhookOverride != "" && cfg.Discord.Webhooks == nil {
		cfg.Discord.Webhooks = map[string]string{"override": o.webhookOverride}
	}
	return cfg, path, nil
}

func (o *globalOptions) applyProfile(cfg *discordconfig.Config) error {
	if o.profile == "" {
		return nil
	}
	if cfg.Profiles == nil {
		return fmt.Errorf("profile %q not found (profiles map is empty)", o.profile)
	}
	profile, ok := cfg.Profiles[o.profile]
	if !ok {
		return fmt.Errorf("profile %q not defined in discord config", o.profile)
	}
	if profile.Discord != nil {
		cfg.Discord = *profile.Discord
	}
	if profile.Client != nil {
		cfg.Client = *profile.Client
		ensureRateLimitDefaults(&cfg.Client)
	}
	o.appliedProfile = o.profile
	return nil
}

func (o *globalOptions) applyEnvironment(cfg *discordconfig.Config) error {
	if o.environment == "" {
		return nil
	}
	if cfg.Environments == nil {
		return fmt.Errorf("environment %q not found (environments map is empty)", o.environment)
	}
	env, ok := cfg.Environments[o.environment]
	if !ok {
		return fmt.Errorf("environment %q not defined in discord config", o.environment)
	}
	if len(env.Webhooks) == 0 {
		return fmt.Errorf("environment %q does not define any webhooks", o.environment)
	}
	cfg.Discord.Webhooks = env.Webhooks
	ensureRateLimitDefaults(&cfg.Client)
	o.appliedEnv = o.environment
	return nil
}

func ensureRateLimitDefaults(cfg *discordconfig.ClientConfig) {
	if cfg.RateLimit.Strategy == "" {
		cfg.RateLimit.Strategy = "adaptive"
	}
	if cfg.RateLimit.BackoffBase == 0 {
		cfg.RateLimit.BackoffBase = time.Second
	}
	if cfg.RateLimit.BackoffMax == 0 {
		cfg.RateLimit.BackoffMax = 60 * time.Second
	}
}

func loadDiscordConfig(path string) (*discordconfig.Config, string, error) {
	candidates := orderedConfigPaths(path)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		expanded := utils.ExpandPath(candidate)
		info, err := os.Stat(expanded)
		if err != nil || info.IsDir() {
			continue
		}
		cfg, err := discordconfig.Load(expanded)
		if err != nil {
			return nil, expanded, fmt.Errorf("failed to load Discord config %s: %w", expanded, err)
		}
		return cfg, expanded, nil
	}
	return discordconfig.Default(), "", nil
}

func orderedConfigPaths(explicit string) []string {
	envPath := os.Getenv("ARC_DISCORD_CONFIG")
	home, _ := os.UserHomeDir()
	defaults := []string{
		filepath.Join(home, ".config", "arc", "discord.yaml"),
		filepath.Join(home, ".arc", "discord.yaml"),
		"discord-config.yaml",
		filepath.Join("config", "discord.yaml"),
	}

	var candidates []string
	seen := map[string]struct{}{}
	push := func(p string) {
		if p == "" {
			return
		}
		normalized := filepath.Clean(p)
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, normalized)
	}

	push(explicit)
	push(envPath)
	for _, d := range defaults {
		push(d)
	}

	if len(candidates) == 0 {
		return defaults
	}
	return candidates
}

func resolveWebhookURL(cfg *discordconfig.Config, opts *globalOptions, name string) (string, error) {
	if opts != nil && opts.webhookOverride != "" {
		return opts.webhookOverride, nil
	}
	if cfg.Discord.Webhooks == nil {
		return "", errors.New("no webhooks configured; provide --webhook-url or update discord.yaml")
	}
	key := name
	if key == "" {
		key = "default"
	}
	if url, ok := cfg.Discord.Webhooks[key]; ok && url != "" {
		return url, nil
	}
	// fallback to first entry for deterministic behaviour
	keys := make([]string, 0, len(cfg.Discord.Webhooks))
	for k := range cfg.Discord.Webhooks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if cfg.Discord.Webhooks[k] != "" {
			return cfg.Discord.Webhooks[k], nil
		}
	}
	return "", fmt.Errorf("webhook %q not found and no fallback available", name)
}

func (o *globalOptions) loadConfigWithInteractions() (*discordconfig.Config, *interactionSettings, string, error) {
	cfg, path, err := o.loadConfig()
	if err != nil {
		return nil, nil, path, err
	}
	settings, err := loadInteractionSettings(path)
	if err != nil {
		return nil, nil, path, err
	}
	return cfg, settings, path, nil
}

type interactionConfigFile struct {
	Discord struct {
		PublicKey string `yaml:"public_key"`
		PublicURL string `yaml:"public_url"`
	} `yaml:"discord"`
	Server       serverConfig       `yaml:"server"`
	Redis        redisConfig        `yaml:"redis"`
	Tunnel       tunnelConfig       `yaml:"tunnel"`
	Interactions interactionsConfig `yaml:"interactions"`
}

func loadInteractionSettings(path string) (*interactionSettings, error) {
	settings := defaultInteractionSettings()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read discord config: %w", err)
		}
		var extras interactionConfigFile
		if err := yaml.Unmarshal(data, &extras); err != nil {
			return nil, fmt.Errorf("parse discord config interactions: %w", err)
		}
		if extras.Discord.PublicKey != "" {
			settings.PublicKey = strings.TrimSpace(extras.Discord.PublicKey)
		}
		if extras.Discord.PublicURL != "" {
			settings.PublicURL = strings.TrimSpace(extras.Discord.PublicURL)
		}
		if extras.Server.ListenAddr != "" {
			settings.Server.ListenAddr = extras.Server.ListenAddr
		}
		if extras.Redis.Addr != "" {
			settings.Redis.Addr = extras.Redis.Addr
		}
		if extras.Redis.DB != 0 {
			settings.Redis.DB = extras.Redis.DB
		}
		if extras.Redis.Password != "" {
			settings.Redis.Password = extras.Redis.Password
		}
		if extras.Redis.ChannelPrefix != "" {
			settings.Redis.ChannelPrefix = extras.Redis.ChannelPrefix
		}
		if extras.Tunnel.Provider != "" {
			settings.Tunnel.Provider = extras.Tunnel.Provider
		}
		if extras.Tunnel.NgrokAuthToken != "" {
			settings.Tunnel.NgrokAuthToken = extras.Tunnel.NgrokAuthToken
		}
		if extras.Interactions.Timeout > 0 {
			settings.Interactions.Timeout = extras.Interactions.Timeout
		}
		if !extras.Interactions.Enabled {
			settings.Interactions.Enabled = false
		}
		mergeHandlerMappings(&settings.Interactions, extras.Interactions.Handlers)
	}

	if val := strings.TrimSpace(os.Getenv(envDiscordPublicKey)); val != "" {
		settings.PublicKey = val
	}
	if val := strings.TrimSpace(os.Getenv(envDiscordPublicURL)); val != "" {
		settings.PublicURL = val
	}
	if val := strings.TrimSpace(os.Getenv(envTunnelProvider)); val != "" {
		settings.Tunnel.Provider = val
	}
	if val := strings.TrimSpace(os.Getenv(envNgrokAuthToken)); val != "" {
		settings.Tunnel.NgrokAuthToken = val
	}
	if settings.Server.ListenAddr == "" {
		settings.Server.ListenAddr = defaultListenAddr
	}
	if settings.Redis.Addr == "" {
		settings.Redis.Addr = defaultRedisAddr
	}
	if settings.Redis.ChannelPrefix == "" {
		settings.Redis.ChannelPrefix = defaultRedisPrefix
	}
	if settings.Interactions.Timeout <= 0 {
		settings.Interactions.Timeout = defaultInteractionTimeout
	}
	ensureHandlerMaps(&settings.Interactions)
	return settings, nil
}

func mergeHandlerMappings(target *interactionsConfig, src handlerMappings) {
	if target == nil {
		return
	}
	if len(src.Commands) > 0 {
		if target.Handlers.Commands == nil {
			target.Handlers.Commands = make(map[string]handlerRoute)
		}
		for k, v := range src.Commands {
			target.Handlers.Commands[strings.ToLower(k)] = v
		}
	}
	if len(src.Components) > 0 {
		if target.Handlers.Components == nil {
			target.Handlers.Components = make(map[string]handlerRoute)
		}
		for k, v := range src.Components {
			target.Handlers.Components[k] = v
		}
	}
	if len(src.Modals) > 0 {
		if target.Handlers.Modals == nil {
			target.Handlers.Modals = make(map[string]handlerRoute)
		}
		for k, v := range src.Modals {
			target.Handlers.Modals[k] = v
		}
	}
	if len(src.Autocomplete) > 0 {
		if target.Handlers.Autocomplete == nil {
			target.Handlers.Autocomplete = make(map[string]handlerRoute)
		}
		for k, v := range src.Autocomplete {
			target.Handlers.Autocomplete[strings.ToLower(k)] = v
		}
	}
}
