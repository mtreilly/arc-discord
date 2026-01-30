package main

import "github.com/spf13/cobra"

func webhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Interact with configured webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			return printFormatted(cmd, map[string]string{"default_webhook": cfg.Discord.Webhooks["default"]})
		},
	}
	return cmd
}
