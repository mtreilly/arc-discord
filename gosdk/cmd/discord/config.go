package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/cmd/discord/output"
	"github.com/yourorg/arc-discord/gosdk/config"
)

type cliContextKey string

const (
	configContextKey cliContextKey = "discord-config"
	outputContextKey cliContextKey = "discord-output"
)

var (
	configFile      string
	overrideToken   string
	overrideWebhook string
	outputFormat    string
)

type cliRuntime struct {
	cfg       *config.Config
	formatter output.Formatter
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discord",
		Short: "Discord SDK CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return err
			}
			if overrideToken != "" {
				cfg.Discord.BotToken = overrideToken
			}
			if overrideWebhook != "" {
				if cfg.Discord.Webhooks == nil {
					cfg.Discord.Webhooks = map[string]string{}
				}
				cfg.Discord.Webhooks["default"] = overrideWebhook
			}
			formatter := output.NewFormatter(outputFormat)
			cmd.SetContext(context.WithValue(cmd.Context(), configContextKey, cfg))
			cmd.SetContext(context.WithValue(cmd.Context(), outputContextKey, formatter))
			if path != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "using config %s\n", path)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&configFile, "config", "", "path to Discord config (YAML)")
	cmd.PersistentFlags().StringVar(&overrideToken, "token", "", "override bot token")
	cmd.PersistentFlags().StringVar(&overrideWebhook, "webhook", "", "override default webhook URL")
	cmd.PersistentFlags().StringVar(&outputFormat, "output", "json", "output format (json/table/yaml)")
	return cmd
}

func loadConfig() (*config.Config, string, error) {
	if configFile != "" {
		cfg, err := config.Load(configFile)
		if err == nil {
			return cfg, configFile, nil
		}
		return nil, "", fmt.Errorf("failed to load config %s: %w", configFile, err)
	}

	paths := []string{
		"discord-config.yaml",
		"discord.yaml",
		filepath.Join("config", "discord.yaml"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			cfg, err := config.Load(p)
			if err != nil {
				return nil, "", err
			}
			return cfg, p, nil
		}
	}

	return config.Default(), "", nil
}

func getConfig(cmd *cobra.Command) *config.Config {
	if cmd == nil {
		return config.Default()
	}
	if value, ok := cmd.Context().Value(configContextKey).(*config.Config); ok && value != nil {
		return value
	}
	return config.Default()
}

func getFormatter(cmd *cobra.Command) output.Formatter {
	if cmd == nil {
		return output.NewFormatter(outputFormat)
	}
	if formatter, ok := cmd.Context().Value(outputContextKey).(output.Formatter); ok && formatter != nil {
		return formatter
	}
	return output.NewFormatter(outputFormat)
}

func printFormatted(cmd *cobra.Command, value interface{}) error {
	formatter := getFormatter(cmd)
	out, err := formatter.Format(value)
	if err != nil {
		return err
	}
	if _, err := cmd.OutOrStdout().Write(out); err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write([]byte("\n"))
	return err
}
