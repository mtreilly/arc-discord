package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func messageEditCmd(opts *globalOptions) *cobra.Command {
	var (
		channelID  string
		messageID  string
		content    string
		embedFiles []string
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing bot-authored message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" || messageID == "" {
				return &arcer.CLIError{Msg: "--channel and --message are required"}
			}
			if strings.TrimSpace(content) == "" && len(embedFiles) == 0 {
				return &arcer.CLIError{Msg: "supply --content or --embed-file when editing a message"}
			}
			return runMessageEdit(cmd, opts, channelID, messageID, content, embedFiles)
		},
		Example: `  arc-discord message edit --channel $CHANNEL --message $MSG --content "Updated text"
  arc-discord message edit --channel $CHANNEL --message $MSG --embed-file embed.json`,
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "Target channel ID")
	cmd.Flags().StringVar(&messageID, "message", "", "Message ID to edit")
	cmd.Flags().StringVar(&content, "content", "", "Replacement content for the message")
	cmd.Flags().StringArrayVar(&embedFiles, "embed-file", nil, "Embed JSON file to include (repeatable)")
	return cmd
}

func runMessageEdit(cmd *cobra.Command, opts *globalOptions, channelID, messageID, content string, embedFiles []string) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}

	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	params := &types.MessageEditParams{}
	if strings.TrimSpace(content) != "" {
		params.Content = content
	}
	if len(embedFiles) > 0 {
		embeds, err := loadEmbeds(embedFiles)
		if err != nil {
			return err
		}
		params.Embeds = embeds
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	if _, err := bot.Messages().EditMessage(ctx, channelID, messageID, params); err != nil {
		return (&arcer.CLIError{Msg: "failed to edit message"}).WithCause(err)
	}

	cmd.Printf("Message %s updated in channel %s\n", messageID, channelID)
	return nil
}

func messageDeleteCmd(opts *globalOptions) *cobra.Command {
	var channelID, messageID string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a bot-authored message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" || messageID == "" {
				return &arcer.CLIError{Msg: "--channel and --message are required"}
			}
			return runMessageDelete(cmd, opts, channelID, messageID)
		},
		Example: `  arc-discord message delete --channel $CHANNEL --message $MSG`,
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Target channel ID")
	cmd.Flags().StringVar(&messageID, "message", "", "Message ID to delete")
	return cmd
}

func runMessageDelete(cmd *cobra.Command, opts *globalOptions, channelID, messageID string) error {
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

	if err := bot.Messages().DeleteMessage(ctx, channelID, messageID); err != nil {
		return (&arcer.CLIError{Msg: "failed to delete message"}).WithCause(err)
	}
	cmd.Printf("Message %s deleted from channel %s\n", messageID, channelID)
	return nil
}

func messageReactCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react",
		Short: "Manage reactions on a message",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(messageReactAddCmd(opts))
	cmd.AddCommand(messageReactRemoveCmd(opts))
	return cmd
}

func messageReactAddCmd(opts *globalOptions) *cobra.Command {
	var channelID, messageID, emoji string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a reaction using the bot identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" || messageID == "" || emoji == "" {
				return &arcer.CLIError{Msg: "--channel, --message, and --emoji are required"}
			}
			return runMessageReact(cmd, opts, channelID, messageID, emoji, true)
		},
		Example: `  arc-discord message react add --channel $CHANNEL --message $MSG --emoji 🔥`,
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().StringVar(&messageID, "message", "", "Message ID")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Emoji to use (unicode or name:id)")
	return cmd
}

func messageReactRemoveCmd(opts *globalOptions) *cobra.Command {
	var channelID, messageID, emoji string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove the bot's reaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == "" || messageID == "" || emoji == "" {
				return &arcer.CLIError{Msg: "--channel, --message, and --emoji are required"}
			}
			return runMessageReact(cmd, opts, channelID, messageID, emoji, false)
		},
		Example: `  arc-discord message react remove --channel $CHANNEL --message $MSG --emoji 🔥`,
	}
	cmd.Flags().StringVar(&channelID, "channel", "", "Channel ID")
	cmd.Flags().StringVar(&messageID, "message", "", "Message ID")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Emoji to remove (unicode or name:id)")
	return cmd
}

func runMessageReact(cmd *cobra.Command, opts *globalOptions, channelID, messageID, emoji string, add bool) error {
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

	messageSvc := bot.Messages()
	if add {
		if err := messageSvc.CreateReaction(ctx, channelID, messageID, emoji); err != nil {
			return (&arcer.CLIError{Msg: "failed to add reaction"}).WithCause(err)
		}
		cmd.Printf("Reaction %s added to message %s\n", emoji, messageID)
		return nil
	}
	if err := messageSvc.DeleteOwnReaction(ctx, channelID, messageID, emoji); err != nil {
		return (&arcer.CLIError{Msg: "failed to remove reaction"}).WithCause(err)
	}
	cmd.Printf("Reaction %s removed from message %s\n", emoji, messageID)
	return nil
}
