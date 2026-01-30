package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	discordconfig "github.com/yourorg/arc-discord/gosdk/config"

	"github.com/yourorg/arc-sdk/output"
)

func configCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect resolved Discord configuration",
		Long: `Manage and debug Discord configuration settings. View the currently loaded configuration,
including bot token (masked for security), webhooks, default guild ID, and active profiles/environments.

Configuration file search order (use first found):
  1. --config flag
  2. VIBE_DISCORD_CONFIG environment variable
  3. ~/.config/vibe/discord.yaml
  4. ~/.vibe/discord.yaml
  5. discord-config.yaml
  6. config/discord.yaml

Configuration Fields:
  • bot_token - Discord bot authentication token
  • application_id - Discord application ID
  • default_guild_id - Default guild to use for guild commands (optional)
  • default_channel_id - Default channel for message send (optional)
  • webhooks - Named webhook URLs for different channels

Environment variables (override config values):
  • DISCORD_BOT_TOKEN - Bot token
  • DISCORD_APPLICATION_ID - Application ID
  • DISCORD_DEFAULT_GUILD_ID - Default guild ID
  • DISCORD_DEFAULT_CHANNEL_ID - Default channel ID
  • DISCORD_WEBHOOK - Default webhook URL
  • DISCORD_RATE_LIMIT_STRATEGY - Rate limit strategy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(configShowCmd(opts))
	return cmd
}

func configShowCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the loaded Discord config and search order",
		Long: `Display the currently loaded Discord configuration including:
  • Configuration file path (shows which file was loaded)
  • Bot token (masked for security)
  • Number of configured webhooks
  • HTTP client timeout
  • Active profile (if using --profile flag)
  • Active environment (if using --env flag)
  • Rate limit strategy (adaptive/reactive/proactive)

Useful for:
  • Verifying configuration is loaded correctly
  • Debugging connection issues
  • Confirming which webhooks are available
  • Checking active profiles/environments`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runConfigShow(cmd, opts, opts.output)
		},
		Example: `Example:
  # Inspect the resolved config in a readable table
  arc-discord config show

Example:
  # Emit JSON for tooling or jq queries
  arc-discord config show --output json

Example:
  # Switch to a named profile before inspecting credentials
  arc-discord config show --profile production

Example:
  # Verify staging webhooks/environment overrides are wiring up
  arc-discord config show --env staging --output yaml

Example:
  # Confirm default guild/channel IDs exist for other commands
  arc-discord config show --output json | jq .config.Discord.DefaultGuildID`,
	}
}

func runConfigShow(cmd *cobra.Command, opts *globalOptions, output output.OutputOptions) error {
	cfg, path, err := opts.loadConfig()
	if err != nil {
		return err
	}

	payload := struct {
		Path   string                `json:"path" yaml:"path"`
		Config *discordconfig.Config `json:"config" yaml:"config"`
	}{
		Path:   path,
		Config: cfg,
	}

	maskedToken := ""
	if cfg != nil && cfg.Discord.BotToken != "" {
		maskedToken = maskToken(cfg.Discord.BotToken)
	}

	summary := map[string]string{
		"path":                path,
		"bot_token":           maskedToken,
		"application_id":      cfg.Discord.ApplicationID,
		"default_guild_id":    valueOrDash(cfg.Discord.DefaultGuildID),
		"default_channel_id":  valueOrDash(cfg.Discord.DefaultChannelID),
		"webhooks":            fmt.Sprintf("%d", len(cfg.Discord.Webhooks)),
		"timeout":             cfg.Client.Timeout.String(),
		"profile":             valueOrDash(opts.appliedProfile),
		"environment":         valueOrDash(opts.appliedEnv),
		"rate_limit_strategy": cfg.Client.RateLimit.Strategy,
	}

	return renderOutput(cmd, output, payload, keyValueTable(summary))
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 6 {
		return "***"
	}
	return fmt.Sprintf("%s…%s", token[:4], token[len(token)-2:])
}

func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
