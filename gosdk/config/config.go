package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Discord SDK configuration
type Config struct {
	Discord      DiscordConfig             `yaml:"discord"`
	Client       ClientConfig              `yaml:"client"`
	Logging      LoggingConfig             `yaml:"logging"`
	Profiles     map[string]ProfileConfig  `yaml:"profiles"`
	Environments map[string]EnvironmentSet `yaml:"environments"`
}

// ProfileConfig represents a named configuration override.
type ProfileConfig struct {
	Discord *DiscordConfig `yaml:"discord"`
	Client  *ClientConfig  `yaml:"client"`
}

// EnvironmentSet swaps webhook maps per environment (e.g., staging/prod).
type EnvironmentSet struct {
	Webhooks map[string]string `yaml:"webhooks"`
}

// DiscordConfig contains Discord-specific configuration
type DiscordConfig struct {
	BotToken        string            `yaml:"bot_token"`
	ApplicationID   string            `yaml:"application_id"`
	DefaultGuildID  string            `yaml:"default_guild_id"`
	DefaultChannelID string            `yaml:"default_channel_id"`
	Webhooks        map[string]string `yaml:"webhooks"`
}

// ClientConfig contains HTTP client configuration
type ClientConfig struct {
	Timeout           time.Duration   `yaml:"timeout"`
	Retries           int             `yaml:"retries"`
	RateLimit         RateLimitConfig `yaml:"rate_limit"`
	RateLimitStrategy string          `yaml:"rate_limit_strategy,omitempty"` // legacy support
}

// RateLimitConfig configures client-side rate limiting
type RateLimitConfig struct {
	Strategy    string        `yaml:"strategy"`
	BackoffBase time.Duration `yaml:"backoff_base"`
	BackoffMax  time.Duration `yaml:"backoff_max"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if cfg.Client.Timeout == 0 {
		cfg.Client.Timeout = 30 * time.Second
	}
	if cfg.Client.Retries == 0 {
		cfg.Client.Retries = 3
	}
	applyRateLimitDefaults(&cfg.Client)
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	return &cfg, nil
}

// Default returns a default configuration
func Default() *Config {
	return &Config{
		Discord: DiscordConfig{
			BotToken:        os.Getenv("DISCORD_BOT_TOKEN"),
			ApplicationID:   os.Getenv("DISCORD_APPLICATION_ID"),
			DefaultGuildID:  os.Getenv("DISCORD_DEFAULT_GUILD_ID"),
			DefaultChannelID: os.Getenv("DISCORD_DEFAULT_CHANNEL_ID"),
			Webhooks: map[string]string{
				"default": os.Getenv("DISCORD_WEBHOOK"),
			},
		},
		Client: ClientConfig{
			Timeout: 30 * time.Second,
			Retries: 3,
			RateLimit: RateLimitConfig{
				Strategy:    getEnvOrDefault("DISCORD_RATE_LIMIT_STRATEGY", "adaptive"),
				BackoffBase: time.Second,
				BackoffMax:  60 * time.Second,
			},
		},
		Logging: LoggingConfig{
			Level:  getEnvOrDefault("DISCORD_LOG_LEVEL", "info"),
			Format: "json",
			Output: "stderr",
		},
	}
}

func applyRateLimitDefaults(cfg *ClientConfig) {
	if cfg.RateLimit.Strategy == "" {
		if cfg.RateLimitStrategy != "" {
			cfg.RateLimit.Strategy = cfg.RateLimitStrategy
		} else {
			cfg.RateLimit.Strategy = getEnvOrDefault("DISCORD_RATE_LIMIT_STRATEGY", "adaptive")
		}
	}
	if cfg.RateLimit.BackoffBase == 0 {
		cfg.RateLimit.BackoffBase = time.Second
	}
	if cfg.RateLimit.BackoffMax == 0 {
		cfg.RateLimit.BackoffMax = 60 * time.Second
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
