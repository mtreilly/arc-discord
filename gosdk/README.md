# Discord Go SDK

Go SDK for Discord API interactions.

## Installation

```bash
go get github.com/yourorg/arc-discord/gosdk
```

## Packages

- **discord/types**: Core types, errors, and models
- **discord/webhook**: Webhook client for sending messages
- **discord/client**: Discord API client (planned)
- **discord/interactions**: Slash commands and components (planned)
- **config**: Configuration management
- **logger**: Structured logging

## Usage

See parent [README.md](../README.md) and [examples/](examples/) for usage examples.

## Testing

```bash
go test ./...
go test -v -cover ./...
```

## Documentation

```bash
go doc -all ./discord/webhook
go doc -all ./discord/types
```
