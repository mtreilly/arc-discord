package main

import "github.com/spf13/cobra"

func channelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "channel",
		Short: "Manage channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			return printFormatted(cmd, cfg.Discord.Webhooks)
		},
	}
}
