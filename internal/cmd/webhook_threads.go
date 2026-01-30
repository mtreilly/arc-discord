package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/client"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func webhookThreadCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Manage webhook threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(webhookThreadCreateCmd(opts))
	cmd.AddCommand(webhookThreadListCmd(opts))
	return cmd
}

func webhookThreadCreateCmd(opts *globalOptions) *cobra.Command {
	var (
		threadName   string
		namedWebhook string
		payloadPath  string
		content      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new forum thread using a webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			if threadName == "" {
				return &arcer.CLIError{Msg: "--thread-name is required"}
			}
			return runWebhookThreadCreate(cmd, opts, threadCreateInput{
				webhookName: namedWebhook,
				threadName:  threadName,
				payloadPath: payloadPath,
				content:     content,
			})
		},
		Example: `Example:
  # Create a forum thread and seed it with JSON payload content
  arc-discord webhook thread create --thread-name "Deploy notes" --payload payload.json

Example:
  # Use inline content with the default webhook
  arc-discord webhook thread create --thread-name "Support" --content "New issue"

Example:
  # Target a specific webhook profile configured for incidents
  arc-discord webhook thread create --webhook incidents --thread-name "Incident 42" --content "Boot logs"`,
	}

	cmd.Flags().StringVar(&namedWebhook, "webhook", "default", "Webhook name from config")
	cmd.Flags().StringVar(&threadName, "thread-name", "", "Name of the forum thread to create")
	cmd.Flags().StringVar(&payloadPath, "payload", "", "Payload JSON for the first message")
	cmd.Flags().StringVar(&content, "content", "", "Message content if no payload is provided")
	return cmd
}

type threadCreateInput struct {
	webhookName string
	threadName  string
	payloadPath string
	content     string
}

func runWebhookThreadCreate(cmd *cobra.Command, opts *globalOptions, input threadCreateInput) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	webhookURL, err := resolveWebhookURL(cfg, opts, input.webhookName)
	if err != nil {
		return err
	}
	dispatcher, err := newWebhookClientFn(cfg, webhookURL)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize webhook client"}).WithCause(err)
	}

	msg, err := buildWebhookMessage(webhookSendInput{
		content:     input.content,
		payloadPath: input.payloadPath,
		threadName:  input.threadName,
	}, "")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	if err := dispatcher.CreateThread(ctx, input.threadName, msg); err != nil {
		return (&arcer.CLIError{Msg: "failed to create thread"}).WithCause(err)
	}

	cmd.Printf("Thread %s creation requested\n", input.threadName)
	return nil
}

func webhookThreadListCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent threads for a channel (requires bot token)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" {
				return &arcer.CLIError{Msg: "--channel is required"}
			}
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runWebhookThreadList(cmd, opts, channelID, limit, opts.output)
		},
		Example: `Example:
  # List recent threads referenced in a forum channel
  arc-discord webhook thread list --channel $CHANNEL --limit 20

Example:
  # Emit YAML to inspect archival timestamps
  arc-discord webhook thread list --channel $CHANNEL --output yaml

Example:
  # Filter results further with jq for automation
  arc-discord webhook thread list --channel $CHANNEL --output json | jq '.[].name'`,
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID to inspect")
	cmd.Flags().IntVar(&limit, "limit", 25, "Number of messages to inspect for thread metadata")
	return cmd
}

func runWebhookThreadList(cmd *cobra.Command, opts *globalOptions, channelID string, limit int, output output.OutputOptions) error {
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

	params := &client.GetChannelMessagesParams{Limit: limit}
	messages, err := bot.Channels().GetChannelMessages(ctx, channelID, params)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to inspect channel"}).WithCause(err)
	}

	rows := [][]string{}
	payload := []map[string]string{}
	for _, m := range messages {
		if m.Thread == nil {
			continue
		}
		info := map[string]string{
			"id":       m.Thread.ID,
			"name":     m.Thread.Name,
			"owner_id": m.Thread.OwnerID,
			"status":   "active",
		}
		if meta := m.Thread.ThreadMetadata; meta != nil && meta.ArchiveTimestamp != nil {
			info["archived_at"] = meta.ArchiveTimestamp.Format(time.RFC3339)
		}
		payload = append(payload, info)
		rows = append(rows, []string{info["id"], info["name"], info["owner_id"], info["status"], info["archived_at"]})
	}

	if len(payload) == 0 {
		cmd.Println("No threads detected in recent history.")
		return nil
	}

	table := &tableData{headers: []string{"ThreadID", "Name", "Owner", "Status", "ArchivedAt"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}
