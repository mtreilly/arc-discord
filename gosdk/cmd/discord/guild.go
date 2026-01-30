package main

import "github.com/spf13/cobra"

func guildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guild",
		Short: "Query guild metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			return printFormatted(cmd, map[string]string{"application_id": cfg.Discord.ApplicationID})
		},
	}
}
