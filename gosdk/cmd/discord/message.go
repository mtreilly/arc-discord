package main

import "github.com/spf13/cobra"

func messageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "message",
		Short: "Send or edit messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			return printFormatted(cmd, map[string]int{"token_length": len(cfg.Discord.BotToken)})
		},
	}
}
