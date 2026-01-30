package main

import "os"

func main() {
	rootCmd := newRootCommand()
	rootCmd.AddCommand(webhookCmd())
	rootCmd.AddCommand(messageCmd())
	rootCmd.AddCommand(channelCmd())
	rootCmd.AddCommand(guildCmd())
	rootCmd.AddCommand(interactionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
