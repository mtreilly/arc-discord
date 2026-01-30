# Webhook Example

This example demonstrates how to use the Discord webhook client.

## Setup

1. Set your Discord webhook URL:
```bash
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
```

2. Run the example:
```bash
go run main.go
```

## What it does

- Sends a simple text message
- Sends a message with a rich embed
- Sends a build success notification with multiple fields

## Features demonstrated

- Simple text messages
- Rich embeds with fields
- Custom colors and timestamps
- Footer and field formatting
- Error handling
- Retry logic
