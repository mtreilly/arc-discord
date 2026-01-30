package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	root := &cobra.Command{
		Use:   "discord",
		Short: "Discord SDK CLI",
	}
	root.AddCommand(webhookCmd(), messageCmd(), channelCmd(), guildCmd(), interactionCmd())

	buf := bytes.NewBuffer(nil)
	root.SetOut(buf)

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
