package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/client"
	"github.com/yourorg/arc-discord/gosdk/discord/types"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func channelHistoryCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID string
		limit     int
		before    string
		after     string
		around    string
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show recent messages for a channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" {
				return &arcer.CLIError{Msg: "--channel is required"}
			}
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runChannelHistory(cmd, opts, channelID, opts.output, limit, before, after, around)
		},
		Example: `Example:
  # Show the latest 20 messages (default format)
  arc-discord channel history --channel $CHANNEL --limit 20

Example:
  # Page backwards from a specific message ID
  arc-discord channel history --channel $CHANNEL --before 12039812398123

Example:
  # Center results around an incident message and output YAML
  arc-discord channel history --channel $CHANNEL --around 12039812398123 --output yaml

Example:
  # Export JSON and pipe into jq for further analysis
  arc-discord channel history --channel $CHANNEL --after 12039812398123 --output json | jq '.[].content'`,
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum messages (1-100)")
	cmd.Flags().StringVar(&before, "before", "", "Message ID to page before")
	cmd.Flags().StringVar(&after, "after", "", "Message ID to page after")
	cmd.Flags().StringVar(&around, "around", "", "Message ID to center results around")
	return cmd
}

func runChannelHistory(cmd *cobra.Command, opts *globalOptions, channelID string, output output.OutputOptions, limit int, before, after, around string) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
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
		return (&arcer.CLIError{Msg: "failed to load channel history"}).WithCause(err)
	}

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
		rows = append(rows, []string{m.ID, entry["author"], entry["timestamp"], entry["content"]})
	}

	table := &tableData{headers: []string{"ID", "Author", "Timestamp", "Content"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}

func safeUser(u *types.User) string {
	if u == nil {
		return "unknown"
	}
	username := u.Username
	if u.Discriminator != "" && u.Discriminator != "0" {
		username = fmt.Sprintf("%s#%s", u.Username, u.Discriminator)
	}
	return username
}

func truncate(val string, limit int) string {
	if len(val) <= limit {
		return val
	}
	return val[:limit-1] + "…"
}
