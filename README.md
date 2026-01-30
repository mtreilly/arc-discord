# arc-discord

Discord integration for the Arc toolkit.

## Features

- **webhook** - Send messages via webhooks
- **message** - Send bot-authenticated messages
- **channel** - Manage channels
- **guild** - Guild operations
- **interaction** - Handle slash commands
- **listen** - Listen for events via gateway
- **server** - Run interaction server

## Installation

```bash
go install github.com/mtreilly/arc-discord@latest
```

## Configuration

Configuration is discovered from:
- `~/.config/arc/discord.yaml`
- `config/discord.yaml`
- `--config` flag

## Usage

```bash
# Send a webhook message
arc-discord webhook send "Deployment complete"

# Post a bot message to a channel
arc-discord message send --channel $CHANNEL_ID "hello agents"

# Get channel info in YAML
arc-discord channel get --channel $CHANNEL_ID --output yaml

# Use a different config
arc-discord webhook send "Test" --config ~/.config/arc/discord_staging.yaml

# Use a named profile
arc-discord message send --profile production "Hello"
```

## SDK

The `gosdk/` directory contains a full Go SDK for Discord:
- REST API client
- Gateway/WebSocket support
- Webhook utilities
- Embed builders
- Rate limiting
- Caching

## License

MIT
