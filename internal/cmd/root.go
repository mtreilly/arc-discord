// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/output"
)

// NewRootCmd creates the root command for arc-discord.
func NewRootCmd() *cobra.Command {
	opts := &globalOptions{}

	cmd := &cobra.Command{
		Use:   "arc-discord",
		Short: "Integrate with Discord bots and webhooks via the Discord SDK",
		Long: `Interact with Discord webhooks and bot endpoints using the Discord SDK.
Configuration is discovered automatically from ~/.config/arc/discord.yaml, config/discord.yaml,
or the file specified with --config. The command family follows the standard --output json|yaml|table pattern
and surfaces webhook as well as authenticated bot workflows.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: `Example:
  # Send a quick webhook notification using the default profile
  arc-discord webhook send "Deployment complete"

Example:
  # Post a bot-authenticated message to a channel
  arc-discord message send --channel $CHANNEL_ID "hello agents"

Example:
  # Inspect channel metadata in YAML
  arc-discord channel get --channel $CHANNEL_ID --output yaml

Example:
  # Override the config path when testing new profiles
  arc-discord webhook send "Smoketest passed" --config ~/.config/arc/discord_staging.yaml`,
	}

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "Path to Discord config file (default: ~/.config/arc/discord.yaml)")
	opts.output.AddOutputFlags(cmd, output.OutputJSON)
	cmd.PersistentFlags().StringVar(&opts.tokenOverride, "token", "", "Override Discord bot token")
	cmd.PersistentFlags().StringVar(&opts.webhookOverride, "webhook-url", "", "Override webhook URL for webhook commands")
	cmd.PersistentFlags().StringVar(&opts.profile, "profile", "", "Use named profile from discord.yaml (switches bot token/webhooks)")
	cmd.PersistentFlags().StringVar(&opts.environment, "env", "", "Use named environment webhooks from discord.yaml")
	cmd.PersistentFlags().StringVar(&opts.rateStrategy, "rate-limit-strategy", "", "Override rate limit strategy: adaptive|reactive|proactive")

	cmd.AddCommand(webhookCmd(opts))
	cmd.AddCommand(messageCmd(opts))
	cmd.AddCommand(channelCmd(opts))
	cmd.AddCommand(guildCmd(opts))
	cmd.AddCommand(configCmd(opts))
	cmd.AddCommand(interactionCmd(opts))
	cmd.AddCommand(serverCmd(opts))
	cmd.AddCommand(agentCmd(opts))

	return cmd
}
