package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/types"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func channelCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Inspect Discord channel metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(channelGetCmd(opts))
	cmd.AddCommand(channelHistoryCmd(opts))
	cmd.AddCommand(channelModifyCmd(opts))
	return cmd
}

func channelGetCmd(opts *globalOptions) *cobra.Command {
	var channelID string

	c := &cobra.Command{
		Use:   "get",
		Short: "Fetch channel details via the bot token",
		Long: `Retrieve detailed metadata about a specific Discord channel, including name, type, topic,
NSFW status, and parent channel. Requires the channel ID (get from channel properties or guild listing).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" {
				return &arcer.CLIError{Msg: "--channel is required", Hint: "pass a Discord channel ID"}
			}
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runChannelGet(cmd, opts, channelID, opts.output)
		},
		Example: `Example:
  # Fetch channel metadata as JSON (default)
  arc-discord channel get --channel 1427555325136867393

Example:
  # Emit YAML for easier diffing or scripting
  arc-discord channel get --channel 1427555325136867393 --output yaml

Example:
  # Show a compact table in the terminal
  arc-discord channel get --channel 1427555325136867393 --output table

Example:
  # Inspect a channel using a staging config file
  arc-discord channel get --channel 1427555325136867393 --config ~/.config/vibe/discord_staging.yaml`,
	}

	c.Flags().StringVar(&channelID, "channel", "", "Channel ID to inspect")

	return c
}

func runChannelGet(cmd *cobra.Command, opts *globalOptions, channelID string, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	ch, err := bot.Channels().GetChannel(ctx, channelID)
	if err != nil {
		return (&arcer.CLIError{Msg: fmt.Sprintf("failed to fetch channel %s", channelID)}).WithCause(err)
	}

	table := keyValueTable(map[string]string{
		"id":        ch.ID,
		"name":      ch.Name,
		"guild_id":  ch.GuildID,
		"type":      channelTypeName(ch.Type),
		"topic":     ch.Topic,
		"nsfw":      fmt.Sprintf("%t", ch.NSFW),
		"parent_id": ch.ParentID,
	})

	return renderOutput(cmd, output, ch, table)
}

func channelModifyCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID string
		name      string
		topic     string
		nsfwFlag  bool
		rateLimit int
	)

	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Update channel metadata (topic/name/flags)",
		Long: `Modify Discord channel properties such as name, topic, NSFW status, and slowmode rate limits.
At least one field must be specified for the update to succeed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" {
				return &arcer.CLIError{Msg: "--channel is required"}
			}
			nsfwChanged := cmd.Flags().Changed("nsfw")
			if name == "" && topic == "" && !nsfwChanged && rateLimit == 0 {
				return &arcer.CLIError{Msg: "specify at least one field to update (--name/--topic/--nsfw/--rate-limit-per-user)"}
			}
			return runChannelModify(cmd, opts, channelID, channelModifyInput{
				name:             name,
				topic:            topic,
				nsfwSet:          nsfwChanged,
				nsfw:             nsfwFlag,
				rateLimitPerUser: rateLimit,
			})
		},
		Example: `Example:
  # Update a channel topic without touching other fields
  arc-discord channel modify --channel 1427555325136867393 --topic "Deployments and CI/CD"

Example:
  # Toggle the NSFW flag explicitly
  arc-discord channel modify --channel 1427555325136867393 --nsfw=false

Example:
  # Apply slowmode to reduce noise
  arc-discord channel modify --channel 1427555325136867393 --rate-limit-per-user 5

Example:
  # Rename + retitle a channel in one call
  arc-discord channel modify --channel 1427555325136867393 --name alerts --topic "System alerts"`,
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID to modify")
	cmd.Flags().StringVar(&name, "name", "", "New channel name")
	cmd.Flags().StringVar(&topic, "topic", "", "New channel topic")
	cmd.Flags().IntVar(&rateLimit, "rate-limit-per-user", 0, "Slowmode rate limit in seconds (0 clears it)")
	cmd.Flags().BoolVar(&nsfwFlag, "nsfw", false, "Mark channel as NSFW (use --nsfw=false to clear)")
	cmd.Flags().Lookup("nsfw").NoOptDefVal = "true"
	return cmd
}

type channelModifyInput struct {
	name             string
	topic            string
	nsfwSet          bool
	nsfw             bool
	rateLimitPerUser int
}

func runChannelModify(cmd *cobra.Command, opts *globalOptions, channelID string, input channelModifyInput) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	params := &types.ModifyChannelParams{}
	if input.name != "" {
		params.Name = input.name
	}
	if input.topic != "" {
		params.Topic = input.topic
	}
	if input.nsfwSet {
		params.NSFW = input.nsfw
	}
	if input.rateLimitPerUser > 0 {
		params.RateLimitPerUser = input.rateLimitPerUser
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	if _, err := bot.Channels().ModifyChannel(ctx, channelID, params); err != nil {
		return (&arcer.CLIError{Msg: "failed to modify channel"}).WithCause(err)
	}
	cmd.Printf("Channel %s updated\n", channelID)
	return nil
}

func channelTypeName(t types.ChannelType) string {
	switch t {
	case types.ChannelTypeGuildText:
		return "guild_text"
	case types.ChannelTypeDM:
		return "dm"
	case types.ChannelTypeGuildVoice:
		return "guild_voice"
	case types.ChannelTypeGroupDM:
		return "group_dm"
	case types.ChannelTypeGuildCategory:
		return "guild_category"
	case types.ChannelTypeGuildNews:
		return "guild_news"
	case types.ChannelTypeGuildStore:
		return "guild_store"
	case types.ChannelTypeGuildNewsThread:
		return "guild_news_thread"
	case types.ChannelTypeGuildPublicThread:
		return "guild_public_thread"
	case types.ChannelTypeGuildPrivateThread:
		return "guild_private_thread"
	case types.ChannelTypeGuildStageVoice:
		return "guild_stage_voice"
	case types.ChannelTypeGuildDirectory:
		return "guild_directory"
	case types.ChannelTypeGuildForum:
		return "guild_forum"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}
