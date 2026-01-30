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

func guildCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guild",
		Short: "Inspect guild metadata via the bot token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(guildGetCmd(opts))
	cmd.AddCommand(guildMembersCmd(opts))
	cmd.AddCommand(guildRolesCmd(opts))
	cmd.AddCommand(guildChannelsCmd(opts))
	return cmd
}

func guildGetCmd(opts *globalOptions) *cobra.Command {
	var guildID string
	var withCounts bool

	c := &cobra.Command{
		Use:   "get",
		Short: "Fetch guild details",
		Long: `Retrieve detailed information about a Discord guild (server), including owner, name, region, and member counts.
Use --with-counts to include approximate member and presence statistics.

Guild ID can be found by:
  1. Enable Developer Mode in Discord settings
  2. Right-click server icon/name in sidebar
  3. Select "Copy Server ID"

If --guild is not provided, uses default_guild_id from discord.yaml (if configured).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runGuildGet(cmd, opts, guildID, withCounts, opts.output)
		},
		Example: `  # Fetch guild info as JSON (uses default_guild_id from config)
  arc-discord guild get

  # Fetch guild info with explicit guild ID
  arc-discord guild get --guild 1427555323857866795

  # Get guild with member counts
  arc-discord guild get --with-counts

  # Output as YAML
  arc-discord guild get --with-counts --output yaml`,
	}

	c.Flags().StringVar(&guildID, "guild", "", "Guild ID to fetch (optional if default_guild_id set in config)")
	c.Flags().BoolVar(&withCounts, "with-counts", false, "Include approximate member presence counts")

	return c
}

func runGuildGet(cmd *cobra.Command, opts *globalOptions, guildID string, withCounts bool, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided guild ID or fall back to config default
	if guildID == "" {
		guildID = cfg.Discord.DefaultGuildID
	}
	if guildID == "" {
		return &arcer.CLIError{Msg: "--guild is required", Hint: "pass a Discord guild ID or set default_guild_id in discord.yaml"}
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	guild, err := bot.Guilds().GetGuild(ctx, guildID, withCounts)
	if err != nil {
		return (&arcer.CLIError{Msg: fmt.Sprintf("failed to fetch guild %s", guildID)}).WithCause(err)
	}

	table := keyValueTable(map[string]string{
		"id":              guild.ID,
		"name":            guild.Name,
		"owner_id":        guild.OwnerID,
		"region":          guild.Region,
		"approx_members":  fmt.Sprintf("%d", guild.ApproximateMemberCount),
		"approx_presence": fmt.Sprintf("%d", guild.ApproximatePresenceCount),
	})

	return renderOutput(cmd, output, guild, table)
}

func guildMembersCmd(opts *globalOptions) *cobra.Command {
	var guildID string
	var limit int
	var after string

	cmd := &cobra.Command{
		Use:   "members",
		Short: "List guild members (requires privileged intent)",
		Long: `List members of a Discord guild with pagination support. Shows user ID, nickname, join date, and role count.

IMPORTANT: Requires the "Server Members Intent" to be enabled:
  1. Go to Discord Developer Portal → Applications → Select your app
  2. Click "Bot" in the left sidebar
  3. Under "Privileged Gateway Intents", enable "Server Members Intent"
  4. Save changes

Pagination:
  • Use --limit to control how many members to return (max 1000, default 50)
  • Use --after to get members after a specific user ID (for pagination)

If --guild is not provided, uses default_guild_id from discord.yaml (if configured).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runGuildMembers(cmd, opts, guildID, limit, after, opts.output)
		},
		Example: `  # List first 50 members (uses default_guild_id from config)
  arc-discord guild members

  # List with custom limit
  arc-discord guild members --limit 100

  # Get next page of members (pagination)
  arc-discord guild members --after 1234567890

  # Export all members to JSON
  arc-discord guild members --limit 1000 > members.json

  # View in table format
  arc-discord guild members --output table`,
	}
	cmd.Flags().StringVar(&guildID, "guild", "", "Guild ID (optional if default_guild_id set in config)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Number of members to return (1-1000)")
	cmd.Flags().StringVar(&after, "after", "", "Only return members after this user ID")
	return cmd
}

func runGuildMembers(cmd *cobra.Command, opts *globalOptions, guildID string, limit int, after string, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided guild ID or fall back to config default
	if guildID == "" {
		guildID = cfg.Discord.DefaultGuildID
	}
	if guildID == "" {
		return &arcer.CLIError{Msg: "--guild is required", Hint: "pass a Discord guild ID or set default_guild_id in discord.yaml"}
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	params := &types.ListMembersParams{Limit: limit}
	if after != "" {
		params.After = after
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	members, err := bot.Guilds().ListGuildMembers(ctx, guildID, params)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to list guild members"}).WithCause(err)
	}

	payload := make([]map[string]string, 0, len(members))
	rows := make([][]string, 0, len(members))
	for _, m := range members {
		entry := map[string]string{
			"id":       safeUser(m.User),
			"nick":     m.Nick,
			"joined":   m.JoinedAt.Format(time.RFC3339),
			"role_cnt": fmt.Sprintf("%d", len(m.Roles)),
		}
		payload = append(payload, entry)
		rows = append(rows, []string{m.User.ID, entry["nick"], entry["joined"], entry["role_cnt"]})
	}

	table := &tableData{headers: []string{"UserID", "Nick", "Joined", "Roles"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}

func guildRolesCmd(opts *globalOptions) *cobra.Command {
	var guildID string
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "List guild roles",
		Long: `List all roles in a Discord guild, including custom roles and permissions.
Shows role ID, name, hex color, and position in the role hierarchy.

Role Information:
  • ID - Unique role identifier
  • Name - Role display name
  • Color - Hex color code (e.g., #3447003)
  • Position - Hierarchy level (higher = more privilege)

If --guild is not provided, uses default_guild_id from discord.yaml (if configured).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runGuildRoles(cmd, opts, guildID, opts.output)
		},
		Example: `  # List roles in table format (uses default_guild_id from config)
  arc-discord guild roles --output table

  # Get roles as JSON
  arc-discord guild roles

  # Export roles as YAML
  arc-discord guild roles --output yaml

  # Find a specific role using grep
  arc-discord guild roles | grep -i moderator`,
	}
	cmd.Flags().StringVar(&guildID, "guild", "", "Guild ID (optional if default_guild_id set in config)")
	return cmd
}

func runGuildRoles(cmd *cobra.Command, opts *globalOptions, guildID string, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided guild ID or fall back to config default
	if guildID == "" {
		guildID = cfg.Discord.DefaultGuildID
	}
	if guildID == "" {
		return &arcer.CLIError{Msg: "--guild is required", Hint: "pass a Discord guild ID or set default_guild_id in discord.yaml"}
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	roles, err := bot.Guilds().GetGuildRoles(ctx, guildID)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to list guild roles"}).WithCause(err)
	}

	rows := make([][]string, 0, len(roles))
	payload := make([]map[string]string, 0, len(roles))
	for _, r := range roles {
		entry := map[string]string{
			"id":    r.ID,
			"name":  r.Name,
			"color": fmt.Sprintf("#%06x", r.Color),
			"pos":   fmt.Sprintf("%d", r.Position),
		}
		payload = append(payload, entry)
		rows = append(rows, []string{r.ID, r.Name, entry["color"], entry["pos"]})
	}

	table := &tableData{headers: []string{"ID", "Name", "Color", "Position"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}

func guildChannelsCmd(opts *globalOptions) *cobra.Command {
	var guildID string
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "List channels within a guild",
		Long: `List all channels in a Discord guild, including text channels, voice channels, categories, and threads.
Shows channel ID, name, type, and parent category for easy organization.

Channel types:
  • guild_text - Text channel for messaging
  • guild_voice - Voice channel for audio/video
  • guild_category - Folder containing other channels
  • guild_public_thread - Public thread in a channel
  • guild_private_thread - Private thread (members only)
  • guild_forum - Forum channel (posts instead of messages)

If --guild is not provided, uses default_guild_id from discord.yaml (if configured).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runGuildChannels(cmd, opts, guildID, opts.output)
		},
		Example: `  # List all channels in table format (uses default_guild_id from config)
  arc-discord guild channels --output table

  # Get channels as JSON (for scripting)
  arc-discord guild channels

  # Export channels as YAML
  arc-discord guild channels --output yaml

  # Use with jq to filter text channels only
  arc-discord guild channels | jq '.[] | select(.type == "guild_text")'`,
	}
	cmd.Flags().StringVar(&guildID, "guild", "", "Guild ID (optional if default_guild_id set in config)")
	return cmd
}

func runGuildChannels(cmd *cobra.Command, opts *globalOptions, guildID string, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided guild ID or fall back to config default
	if guildID == "" {
		guildID = cfg.Discord.DefaultGuildID
	}
	if guildID == "" {
		return &arcer.CLIError{Msg: "--guild is required", Hint: "pass a Discord guild ID or set default_guild_id in discord.yaml"}
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	channels, err := bot.Guilds().GetGuildChannels(ctx, guildID)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to list guild channels"}).WithCause(err)
	}

	rows := make([][]string, 0, len(channels))
	payload := make([]map[string]string, 0, len(channels))
	for _, ch := range channels {
		entry := map[string]string{
			"id":     ch.ID,
			"name":   ch.Name,
			"type":   channelTypeName(ch.Type),
			"parent": ch.ParentID,
		}
		payload = append(payload, entry)
		rows = append(rows, []string{ch.ID, ch.Name, entry["type"], entry["parent"]})
	}

	table := &tableData{headers: []string{"ID", "Name", "Type", "Parent"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}
