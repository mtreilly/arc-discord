package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/types"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func interactionCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interaction",
		Short: "Manage slash commands and interactions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(interactionListCmd(opts))
	cmd.AddCommand(interactionRegisterCmd(opts))
	cmd.AddCommand(interactionDeleteCmd(opts))
	return cmd
}

func interactionListCmd(opts *globalOptions) *cobra.Command {
	var guildID string
	var applicationID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered application commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runInteractionList(cmd, opts, applicationID, guildID, opts.output)
		},
		Example: `  # List global commands
	arc-discord interaction list --application-id $APP

	# List guild-scoped commands
	arc-discord interaction list --guild $GUILD`,
	}
	cmd.Flags().StringVar(&guildID, "guild", "", "Guild ID (omit for global commands)")
	cmd.Flags().StringVar(&applicationID, "application-id", "", "Override application ID (default from config)")
	return cmd
}

func runInteractionList(cmd *cobra.Command, opts *globalOptions, appID, guildID string, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	if appID == "" {
		appID = cfg.Discord.ApplicationID
	}
	if strings.TrimSpace(appID) == "" {
		return &arcer.CLIError{Msg: "application ID not configured", Hint: "set discord.application_id or pass --application-id"}
	}
	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	commandsSvc := bot.ApplicationCommands(appID)
	var cmds []*types.ApplicationCommand
	if guildID == "" {
		cmds, err = commandsSvc.GetGlobalApplicationCommands(ctx)
	} else {
		cmds, err = commandsSvc.GetGuildApplicationCommands(ctx, guildID)
	}
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to list application commands"}).WithCause(err)
	}

	payload := make([]map[string]string, 0, len(cmds))
	rows := make([][]string, 0, len(cmds))
	for _, c := range cmds {
		entry := map[string]string{
			"id":          c.ID,
			"name":        c.Name,
			"description": c.Description,
			"type":        fmt.Sprintf("%d", c.Type),
		}
		payload = append(payload, entry)
		rows = append(rows, []string{c.ID, c.Name, entry["type"], entry["description"]})
	}

	table := &tableData{headers: []string{"ID", "Name", "Type", "Description"}, rows: rows}
	return renderOutput(cmd, output, payload, table)
}

func interactionRegisterCmd(opts *globalOptions) *cobra.Command {
	var (
		defPath       string
		guildID       string
		applicationID string
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register or overwrite an application command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if defPath == "" {
				return &arcer.CLIError{Msg: "--file is required", Hint: "provide a JSON definition for the application command"}
			}
			return runInteractionRegister(cmd, opts, defPath, applicationID, guildID)
		},
		Example: `  arc-discord interaction register --file slash.json
  arc-discord interaction register --file slash.json --guild $GUILD`,
	}

	cmd.Flags().StringVar(&defPath, "file", "", "Path to JSON definition (types.ApplicationCommand)")
	cmd.Flags().StringVar(&guildID, "guild", "", "Optional guild ID for guild-scoped command")
	cmd.Flags().StringVar(&applicationID, "application-id", "", "Override application ID")
	return cmd
}

func runInteractionRegister(cmd *cobra.Command, opts *globalOptions, path, appID, guildID string) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	if appID == "" {
		appID = cfg.Discord.ApplicationID
	}
	if strings.TrimSpace(appID) == "" {
		return &arcer.CLIError{Msg: "application ID not configured", Hint: "set discord.application_id or pass --application-id"}
	}
	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return (&arcer.CLIError{Msg: fmt.Sprintf("failed to read %s", path)}).WithCause(err)
	}
	var command types.ApplicationCommand
	if err := json.Unmarshal(data, &command); err != nil {
		return (&arcer.CLIError{Msg: "invalid application command JSON"}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	commandsSvc := bot.ApplicationCommands(appID)
	var created *types.ApplicationCommand
	if guildID == "" {
		created, err = commandsSvc.CreateGlobalApplicationCommand(ctx, &command)
	} else {
		created, err = commandsSvc.CreateGuildApplicationCommand(ctx, guildID, &command)
	}
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to register command"}).WithCause(err)
	}

	cmd.Printf("Command %s (%s) registered\n", created.Name, created.ID)
	return nil
}

func interactionDeleteCmd(opts *globalOptions) *cobra.Command {
	var (
		applicationID string
		guildID       string
		commandID     string
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an application command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandID == "" {
				return &arcer.CLIError{Msg: "--command-id is required"}
			}
			return runInteractionDelete(cmd, opts, applicationID, guildID, commandID)
		},
		Example: `  arc-discord interaction delete --application-id $APP --command-id $CMD
	arc-discord interaction delete --application-id $APP --guild $GUILD --command-id $CMD`,
	}

	cmd.Flags().StringVar(&applicationID, "application-id", "", "Override application ID (default from config)")
	cmd.Flags().StringVar(&guildID, "guild", "", "Guild ID when deleting guild-scoped commands")
	cmd.Flags().StringVar(&commandID, "command-id", "", "Application command ID to delete")
	return cmd
}

func runInteractionDelete(cmd *cobra.Command, opts *globalOptions, appID, guildID, commandID string) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	if appID == "" {
		appID = cfg.Discord.ApplicationID
	}
	if strings.TrimSpace(appID) == "" {
		return &arcer.CLIError{Msg: "application ID not configured", Hint: "set discord.application_id or pass --application-id"}
	}
	bot, err := newBotClientFn(cfg, opts.tokenOverride)
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to initialize Discord bot client"}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	commandsSvc := bot.ApplicationCommands(appID)
	if guildID == "" {
		err = commandsSvc.DeleteGlobalApplicationCommand(ctx, commandID)
	} else {
		err = commandsSvc.DeleteGuildApplicationCommand(ctx, guildID, commandID)
	}
	if err != nil {
		return (&arcer.CLIError{Msg: "failed to delete application command"}).WithCause(err)
	}

	cmd.Printf("Command %s deleted\n", commandID)
	return nil
}
