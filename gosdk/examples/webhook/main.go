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
	// Get webhook URL from environment
	webhookURL := os.Getenv("DISCORD_WEBHOOK")
	if webhookURL == "" {
		log.Fatal("DISCORD_WEBHOOK environment variable is required")
	}

	// Create webhook client
	opts := []webhook.Option{
		webhook.WithMaxRetries(3),
		webhook.WithTimeout(30 * time.Second),
	}

	if strategy := os.Getenv("DISCORD_RATE_LIMIT_STRATEGY"); strategy != "" {
		fmt.Printf("Using rate limit strategy: %s\n", strategy)
		opts = append(opts, webhook.WithStrategyName(strategy))
	}

	client, err := webhook.NewClient(webhookURL, opts...)
	if err != nil {
		log.Fatalf("Failed to create webhook client: %v", err)
	}

	ctx := context.Background()

	// Example 1: Send simple text message
	fmt.Println("Sending simple message...")
	if err := client.SendSimple(ctx, "Hello from Discord Go SDK!"); err != nil {
		log.Fatalf("Failed to send simple message: %v", err)
	}
	fmt.Println("✓ Simple message sent")

	// Example 2: Send message with embed
	fmt.Println("\nSending message with embed...")
	msg := &types.WebhookMessage{
		Content:  "Check out this embed:",
		Username: "SDK Bot",
		Embeds: []types.Embed{
			{
				Title:       "Discord Go SDK",
				Description: "A production-ready Go SDK for Discord interactions",
				Color:       0x5865F2, // Discord blurple
				Timestamp:   &[]time.Time{time.Now()}[0],
				Fields: []types.EmbedField{
					{
						Name:   "Features",
						Value:  "✓ Webhooks\n✓ Bot API\n✓ Slash Commands",
						Inline: true,
					},
					{
						Name:   "Status",
						Value:  "🚧 In Development",
						Inline: true,
					},
				},
				Footer: &types.EmbedFooter{
					Text: "Powered by Go",
				},
			},
		},
	}

	if err := client.Send(ctx, msg); err != nil {
		log.Fatalf("Failed to send embed message: %v", err)
	}
	fmt.Println("✓ Embed message sent")

	// Example 3: Send success notification
	fmt.Println("\nSending success notification...")
	successMsg := &types.WebhookMessage{
		Embeds: []types.Embed{
			{
				Title:       "✅ Build Successful",
				Description: "All tests passed successfully",
				Color:       0x00FF00, // Green
				Timestamp:   &[]time.Time{time.Now()}[0],
				Fields: []types.EmbedField{
					{
						Name:   "Build #",
						Value:  "123",
						Inline: true,
					},
					{
						Name:   "Duration",
						Value:  "2m 34s",
						Inline: true,
					},
					{
						Name:   "Coverage",
						Value:  "87.5%",
						Inline: true,
					},
				},
			},
		},
	}

	if err := client.Send(ctx, successMsg); err != nil {
		log.Fatalf("Failed to send success notification: %v", err)
	}
	fmt.Println("✓ Success notification sent")

	fmt.Println("\n✅ All examples completed successfully!")
}
