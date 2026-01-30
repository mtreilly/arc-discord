package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func messageListCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID string
		limit     int
		before    string
		after     string
		around    string
		contains  string
		fromUser  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent messages from a channel",
		Long: `List recent messages from a Discord channel.

If --channel is not provided, uses default_channel_id from discord.yaml (if configured).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runMessageList(cmd, opts, channelID, opts.output, limit, before, after, around, contains, fromUser)
		},
		Example: `Example:
  # List the latest 20 messages using the default channel
  arc-discord message list

Example:
  # List the latest 10 messages with default formatting
  arc-discord message list --channel $CHANNEL --limit 10

Example:
  # Page backward from a message ID and show table output
  arc-discord message list --before 123456789 --output table

Example:
  # Filter to messages that contain a keyword
  arc-discord message list --contains incident

Example:
  # Show only messages from a specific bot user
  arc-discord message list --from 123456789012345678 --output json`,
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID to inspect (optional if default_channel_id set in config)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum messages (1-100)")
	cmd.Flags().StringVar(&before, "before", "", "Message ID to page before")
	cmd.Flags().StringVar(&after, "after", "", "Message ID to page after")
	cmd.Flags().StringVar(&around, "around", "", "Message ID to center results around")
	cmd.Flags().StringVar(&contains, "contains", "", "Only include messages containing this substring")
	cmd.Flags().StringVar(&fromUser, "from", "", "Only include messages from a specific author ID")
	return cmd
}

func runMessageList(cmd *cobra.Command, opts *globalOptions, channelID string, output output.OutputOptions, limit int, before, after, around, contains, fromUser string) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided channel ID or fall back to config default
	if channelID == "" {
		channelID = cfg.Discord.DefaultChannelID
	}
	if channelID == "" {
		return &arcer.CLIError{Msg: "--channel is required", Hint: "pass a Discord channel ID or set default_channel_id in discord.yaml"}
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	params := &client.GetChannelMessagesParams{Limit: limit}
	if before != "" {
		params.Before = before
	}
	if after != "" {
		params.After = after
	}
	if around != "" {
		params.Around = around
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	messages, err := bot.Channels().GetChannelMessages(ctx, channelID, params)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to load messages"}).WithCause(err)
	}
	messages = filterMessages(messages, contains, fromUser)

	payload := make([]map[string]string, 0, len(messages))
	rows := make([][]string, 0, len(messages))
	for _, m := range messages {
		entry := map[string]string{
			"id":        m.ID,
			"author":    safeUser(m.Author),
			"timestamp": m.Timestamp.Format(time.RFC3339),
			"content":   truncate(m.Content, 80),
		}
		payload = append(payload, entry)
		rows = append(rows, []string{entry["id"], entry["author"], entry["timestamp"], entry["content"]})
	}

	table := &tableData{headers: []string{"ID", "Author", "Timestamp", "Content"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}

func filterMessages(messages []*types.Message, contains, fromUser string) []*types.Message {
	var filtered []*types.Message
	contains = strings.TrimSpace(contains)
	fromUser = strings.TrimSpace(fromUser)

	for _, m := range messages {
		if fromUser != "" {
			if m.Author == nil || m.Author.ID != fromUser {
				continue
			}
		}
		if contains != "" && !strings.Contains(strings.ToLower(m.Content), strings.ToLower(contains)) {
			continue
		}
		filtered = append(filtered, m)
	}

	return filtered
}
