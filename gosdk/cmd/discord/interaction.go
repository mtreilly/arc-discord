package main

import "github.com/spf13/cobra"

func interactionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interaction",
		Short: "Respond to interactions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig(cmd)
			if len(cfg.Discord.Webhooks) > 0 {
				return printFormatted(cmd, map[string]string{"webhook": cfg.Discord.Webhooks["default"]})
			}
			return printFormatted(cmd, map[string]string{"error": "no webhook configured"})
		},
	}
}
