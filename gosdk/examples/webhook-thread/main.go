package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/discord/webhook"
)

func main() {
	webhookURL := os.Getenv("DISCORD_WEBHOOK")
	if webhookURL == "" {
		log.Fatal("DISCORD_WEBHOOK environment variable is required")
	}

	client, err := webhook.NewClient(webhookURL)
	if err != nil {
		log.Fatalf("failed to create webhook client: %v", err)
	}

	ctx := context.Background()

	// Send into an existing thread when DISCORD_WEBHOOK_THREAD_ID is set
	if threadID := os.Getenv("DISCORD_WEBHOOK_THREAD_ID"); threadID != "" {
		fmt.Printf("Sending update to thread %s...\n", threadID)
		msg := &types.WebhookMessage{
			Content: fmt.Sprintf("Thread update at %s", time.Now().Format(time.RFC822)),
		}
		if err := client.SendToThread(ctx, threadID, msg); err != nil {
			log.Fatalf("SendToThread failed: %v", err)
		}
		fmt.Println("✓ Thread message sent")
	} else {
		fmt.Println("Set DISCORD_WEBHOOK_THREAD_ID to post into an existing thread.")
	}

	// Forum channels: setting thread_name creates a brand-new thread
	threadName := os.Getenv("DISCORD_WEBHOOK_THREAD_NAME")
	if threadName == "" {
		threadName = fmt.Sprintf("sdk-thread-%d", time.Now().Unix())
	}

	fmt.Printf("Creating forum thread %q...\n", threadName)
	createMsg := &types.WebhookMessage{
		Content:    "Opening post from the Go SDK",
		ThreadName: threadName,
		Embeds: []types.Embed{
			{
				Title:       "Thread starter",
				Description: "Demonstrates CreateThread via webhook.",
				Timestamp:   ptr(time.Now()),
			},
		},
	}

	if err := client.CreateThread(ctx, threadName, createMsg); err != nil {
		log.Fatalf("CreateThread failed: %v", err)
	}

	fmt.Println("✓ Thread creation request sent (requires forum-capable webhook channel).")
}

func ptr[T any](v T) *T {
	return &v
}
