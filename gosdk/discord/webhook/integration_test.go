//go:build integration

package webhook

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegrationWebhookSend(t *testing.T) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK")
	if webhookURL == "" {
		t.Skip("DISCORD_WEBHOOK not set; skipping integration test")
	}

	client, err := NewClient(webhookURL)
	if err != nil {
		t.Fatalf("failed to create webhook client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.SendSimple(ctx, "Integration test message from gosdk"); err != nil {
		t.Fatalf("SendSimple failed: %v", err)
	}
}
