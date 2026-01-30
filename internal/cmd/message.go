package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/types"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func messageCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send messages with the authenticated Discord bot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(messageSendCmd(opts))
	cmd.AddCommand(messageEditCmd(opts))
	cmd.AddCommand(messageDeleteCmd(opts))
	cmd.AddCommand(messageReactCmd(opts))
	cmd.AddCommand(messageListCmd(opts))
	return cmd
}

func messageSendCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID   string
		payloadPath string
		content     string
	)

	c := &cobra.Command{
		Use:   "send",
		Short: "Send a message to a Discord channel using the bot token",
		Long: `Send a message to a Discord channel using authenticated bot credentials.
Supports both simple text content and complex messages with rich embeds, fields, and metadata.
Messages can be sent as plain text or loaded from a JSON payload file for advanced formatting.

If --channel is not provided, uses default_channel_id from discord.yaml (if configured).

Payload Structure (types.MessageCreateParams):
  {
    "content": "Message text",
    "embeds": [
      {
        "title": "Embed Title",
        "description": "Embed Description",
        "color": 3447003,
        "fields": [{"name": "Field", "value": "Value", "inline": false}],
        "author": {"name": "Author Name", "icon_url": "..."},
        "footer": {"text": "Footer text", "icon_url": "..."}
      }
    ]
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runMessageSend(cmd, opts, messageSendInput{
				channelID:   channelID,
				payloadPath: payloadPath,
				content:     content,
				output:      opts.output,
			})
		},
		Example: `Example:
  # Send a quick text update using the default channel
  arc-discord message send --content "Hello from vibe CLI!"

Example:
  # Target a specific channel ID explicitly
  arc-discord message send --channel 1427555325136867393 --content "Hello!"

Example:
  # Load an embed-driven payload from disk
  arc-discord message send --payload advanced_message.json

Example:
  # Combine inline content with YAML output for logging
  arc-discord message send --content "Test" --output yaml

Example:
  # Use a staging profile when testing bot credentials
  arc-discord message send --content \"Smoke test\" --profile staging`,
	}

	c.Flags().StringVar(&channelID, "channel", "", "Target channel ID (optional if default_channel_id set in config)")
	c.Flags().StringVar(&payloadPath, "payload", "", "Path to JSON payload for types.MessageCreateParams")
	c.Flags().StringVar(&content, "content", "", "Message content when not using --payload")

	return c
}

type messageSendInput struct {
	channelID   string
	payloadPath string
	content     string
	output      output.OutputOptions
}

func runMessageSend(cmd *cobra.Command, opts *globalOptions, in messageSendInput) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	// Use provided channel ID or fall back to config default
	if in.channelID == "" {
		in.channelID = cfg.Discord.DefaultChannelID
	}
	if in.channelID == "" {
		return &arcer.CLIError{Msg: "--channel is required", Hint: "pass a Discord channel ID or set default_channel_id in discord.yaml"}
	}

	params, err := buildMessageParams(in)
	if err != nil {
		return err
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	msg, err := bot.Messages().CreateMessage(ctx, in.channelID, params)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to send Discord message"}).WithCause(err)
	}

	data := map[string]string{
		"message_id": msg.ID,
		"channel_id": msg.ChannelID,
		"guild_id":   msg.GuildID,
		"timestamp":  msg.Timestamp.Format(time.RFC3339),
		"status":     "sent",
	}

	return renderOutput(cmd, in.output, msg, keyValueTable(data))
}

func buildMessageParams(in messageSendInput) (*types.MessageCreateParams, error) {
	if in.payloadPath != "" {
		data, err := os.ReadFile(in.payloadPath)
		if err != nil {
			return nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to read payload %s", in.payloadPath)}).WithCause(err)
		}
		var params types.MessageCreateParams
		if err := json.Unmarshal(data, &params); err != nil {
			return nil, (&arcer.CLIError{Msg: "payload must be valid JSON for types.MessageCreateParams"}).WithCause(err)
		}
		return &params, nil
	}
	if in.content == "" {
		return nil, &arcer.CLIError{Msg: "provide --content or --payload"}
	}
	return &types.MessageCreateParams{Content: in.content}, nil
}
