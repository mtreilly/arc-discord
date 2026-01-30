package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// PrereqStatus represents the status of a single prerequisite check.
type PrereqStatus int

const (
	PrereqOK PrereqStatus = iota
	PrereqMissing
	PrereqInvalid
	PrereqUnreachable
)

func (s PrereqStatus) String() string {
	switch s {
	case PrereqOK:
		return "OK"
	case PrereqMissing:
		return "MISSING"
	case PrereqInvalid:
		return "INVALID"
	case PrereqUnreachable:
		return "UNREACHABLE"
	default:
		return "UNKNOWN"
	}
}

// PrereqCheck represents a single prerequisite check result.
type PrereqCheck struct {
	Name        string
	Status      PrereqStatus
	Value       string // Current value (if any)
	Required    bool
	Description string
	HowToFix    string
	Example     string
	EnvVar      string // Environment variable alternative
	ConfigKey   string // Config file key
}

// PrereqReport contains all prerequisite check results.
type PrereqReport struct {
	ConfigPath string
	Checks     []PrereqCheck
	AllPassed  bool
}

// ServerPrereqChecker validates all prerequisites for running the Discord server.
type ServerPrereqChecker struct {
	opts      *globalOptions
	overrides serverStartOptions
}

// NewServerPrereqChecker creates a new prerequisite checker.
func NewServerPrereqChecker(opts *globalOptions, overrides serverStartOptions) *ServerPrereqChecker {
	return &ServerPrereqChecker{
		opts:      opts,
		overrides: overrides,
	}
}

// Check runs all prerequisite checks and returns a report.
func (c *ServerPrereqChecker) Check(ctx context.Context) (*PrereqReport, error) {
	report := &PrereqReport{
		AllPassed: true,
	}

	// Step 1: Check config file
	cfg, extra, cfgPath, err := c.opts.loadConfigWithInteractions()
	report.ConfigPath = cfgPath

	configCheck := c.checkConfigFile(cfgPath, err)
	report.Checks = append(report.Checks, configCheck)
	if configCheck.Status != PrereqOK {
		report.AllPassed = false
		// Can't continue without config
		return report, nil
	}

	// Apply overrides for subsequent checks
	if c.overrides.ListenAddr != "" {
		extra.Server.ListenAddr = c.overrides.ListenAddr
	}
	if c.overrides.PublicURL != "" {
		extra.PublicURL = c.overrides.PublicURL
	}
	if c.overrides.RedisAddr != "" {
		extra.Redis.Addr = c.overrides.RedisAddr
	}
	if c.overrides.TunnelProvider != "" {
		extra.Tunnel.Provider = c.overrides.TunnelProvider
	}
	if c.overrides.NgrokToken != "" {
		extra.Tunnel.NgrokAuthToken = c.overrides.NgrokToken
	}

	// Step 2: Check public key (required for signature verification)
	publicKeyCheck := c.checkPublicKey(extra.PublicKey, c.overrides.DryRun)
	report.Checks = append(report.Checks, publicKeyCheck)
	if publicKeyCheck.Status != PrereqOK && publicKeyCheck.Required {
		report.AllPassed = false
	}

	// Step 3: Check Redis connectivity
	redisCheck := c.checkRedis(ctx, extra.Redis)
	report.Checks = append(report.Checks, redisCheck)
	if redisCheck.Status != PrereqOK {
		report.AllPassed = false
	}

	// Step 4: Check interactions configuration
	interactionsCheck := c.checkInteractions(extra.Interactions)
	report.Checks = append(report.Checks, interactionsCheck)
	if interactionsCheck.Status != PrereqOK {
		report.AllPassed = false
	}

	// Step 5: Check tunnel configuration (if needed and no public URL)
	tunnelCheck := c.checkTunnel(extra.Tunnel, extra.PublicURL)
	report.Checks = append(report.Checks, tunnelCheck)
	// Tunnel is optional, don't fail if missing

	// Step 6: Check application ID (needed for responding to interactions)
	appIDCheck := c.checkApplicationID(cfg.Discord.ApplicationID)
	report.Checks = append(report.Checks, appIDCheck)
	if appIDCheck.Status != PrereqOK {
		report.AllPassed = false
	}

	return report, nil
}

func (c *ServerPrereqChecker) checkConfigFile(path string, loadErr error) PrereqCheck {
	check := PrereqCheck{
		Name:        "Configuration File",
		Required:    true,
		Description: "Discord configuration file with server settings",
		ConfigKey:   "~/.config/vibe/discord.yaml",
		EnvVar:      "VIBE_DISCORD_CONFIG",
	}

	if loadErr != nil {
		check.Status = PrereqInvalid
		check.Value = fmt.Sprintf("Error: %v", loadErr)
		check.HowToFix = "Fix the syntax error in your configuration file"
		check.Example = ""
		return check
	}

	if path == "" {
		check.Status = PrereqMissing
		check.HowToFix = "Create a Discord configuration file"
		check.Example = `# Create ~/.config/vibe/discord.yaml with:

discord:
  bot_token: "YOUR_BOT_TOKEN"
  application_id: "YOUR_APP_ID"
  public_key: "YOUR_PUBLIC_KEY"

server:
  listen_addr: "127.0.0.1:8080"

redis:
  addr: "127.0.0.1:6379"

interactions:
  enabled: true
  handlers:
    commands:
      ping:
        agent: "default"
        description: "Ping the bot"`
		return check
	}

	check.Status = PrereqOK
	check.Value = path
	return check
}

func (c *ServerPrereqChecker) checkPublicKey(publicKey string, dryRun bool) PrereqCheck {
	check := PrereqCheck{
		Name:        "Discord Public Key",
		Required:    !dryRun,
		Description: "Used to verify Discord interaction signatures",
		ConfigKey:   "discord.public_key",
		EnvVar:      envDiscordPublicKey,
	}

	if dryRun {
		check.Status = PrereqOK
		check.Value = "(skipped - dry-run mode)"
		return check
	}

	if publicKey == "" {
		check.Status = PrereqMissing
		check.HowToFix = "Add your Discord application's public key"
		check.Example = fmt.Sprintf(`# Option 1: Add to discord.yaml
discord:
  public_key: "YOUR_PUBLIC_KEY_HERE"

# Option 2: Set environment variable
export %s="YOUR_PUBLIC_KEY_HERE"

# Find your public key at:
# https://discord.com/developers/applications/YOUR_APP_ID/information
# Look for "PUBLIC KEY" in the General Information section`, envDiscordPublicKey)
		return check
	}

	// Basic validation - public key should be 64 hex characters
	if len(publicKey) != 64 {
		check.Status = PrereqInvalid
		check.Value = fmt.Sprintf("%d characters (expected 64)", len(publicKey))
		check.HowToFix = "The public key should be exactly 64 hexadecimal characters"
		check.Example = `# Copy the full PUBLIC KEY from Discord Developer Portal
# It should look like: a1b2c3d4e5f6...64 characters total`
		return check
	}

	check.Status = PrereqOK
	check.Value = publicKey[:8] + "..." + publicKey[len(publicKey)-8:]
	return check
}

func (c *ServerPrereqChecker) checkRedis(ctx context.Context, cfg redisConfig) PrereqCheck {
	check := PrereqCheck{
		Name:        "Redis Connection",
		Required:    true,
		Description: "Message broker for routing interactions to agents",
		ConfigKey:   "redis.addr",
		EnvVar:      envDefaultRedisAddr,
	}

	if cfg.Addr == "" {
		check.Status = PrereqMissing
		check.HowToFix = "Configure Redis address"
		check.Example = fmt.Sprintf(`# Option 1: Add to discord.yaml
redis:
  addr: "127.0.0.1:6379"
  # password: "optional"
  # db: 0

# Option 2: Set environment variable
export %s="127.0.0.1:6379"

# Start Redis locally:
# macOS:   brew install redis && brew services start redis
# Ubuntu:  sudo apt install redis-server && sudo systemctl start redis
# Docker:  docker run -d -p 6379:6379 redis:alpine`, envDefaultRedisAddr)
		return check
	}

	// Try to connect
	client := redis.NewClient(newRedisOptions(cfg))
	defer client.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		check.Status = PrereqUnreachable
		check.Value = cfg.Addr
		check.HowToFix = fmt.Sprintf("Cannot connect to Redis at %s", cfg.Addr)
		check.Example = fmt.Sprintf(`# Check if Redis is running:
redis-cli -h %s ping

# Start Redis:
# macOS:   brew services start redis
# Ubuntu:  sudo systemctl start redis
# Docker:  docker run -d -p 6379:6379 redis:alpine

# If Redis requires authentication, add to discord.yaml:
redis:
  addr: "%s"
  password: "your-password"`, parseRedisHost(cfg.Addr), cfg.Addr)
		return check
	}

	check.Status = PrereqOK
	check.Value = cfg.Addr
	return check
}

func (c *ServerPrereqChecker) checkInteractions(cfg interactionsConfig) PrereqCheck {
	check := PrereqCheck{
		Name:        "Interaction Handlers",
		Required:    true,
		Description: "Defines which slash commands route to which agents",
		ConfigKey:   "interactions.handlers",
	}

	if !cfg.Enabled {
		check.Status = PrereqInvalid
		check.Value = "disabled"
		check.HowToFix = "Enable interactions in your configuration"
		check.Example = `# Add to discord.yaml:
interactions:
  enabled: true
  timeout: 30s
  handlers:
    commands:
      ping:
        agent: "default"
        description: "Ping the bot"`
		return check
	}

	totalHandlers := len(cfg.Handlers.Commands) +
		len(cfg.Handlers.Components) +
		len(cfg.Handlers.Modals) +
		len(cfg.Handlers.Autocomplete)

	if totalHandlers == 0 {
		check.Status = PrereqMissing
		check.HowToFix = "Define at least one interaction handler"
		check.Example = `# Add handlers to discord.yaml:
interactions:
  enabled: true
  timeout: 30s
  handlers:
    commands:
      # Route /ping to the "default" agent
      ping:
        agent: "default"
        description: "Ping the bot"

      # Route /ask to the "claude" agent
      ask:
        agent: "claude"
        description: "Ask Claude a question"

      # Route /build to the "builder" agent
      build:
        agent: "builder"
        description: "Trigger a build"

    # Optional: button/select menu handlers
    components:
      approve_button:
        agent: "reviewer"

    # Optional: modal submit handlers
    modals:
      feedback_form:
        agent: "feedback"`
		return check
	}

	// Collect handler summary
	var parts []string
	if n := len(cfg.Handlers.Commands); n > 0 {
		parts = append(parts, fmt.Sprintf("%d command(s)", n))
	}
	if n := len(cfg.Handlers.Components); n > 0 {
		parts = append(parts, fmt.Sprintf("%d component(s)", n))
	}
	if n := len(cfg.Handlers.Modals); n > 0 {
		parts = append(parts, fmt.Sprintf("%d modal(s)", n))
	}
	if n := len(cfg.Handlers.Autocomplete); n > 0 {
		parts = append(parts, fmt.Sprintf("%d autocomplete", n))
	}

	check.Status = PrereqOK
	check.Value = strings.Join(parts, ", ")
	return check
}

func (c *ServerPrereqChecker) checkTunnel(cfg tunnelConfig, publicURL string) PrereqCheck {
	check := PrereqCheck{
		Name:        "Public URL / Tunnel",
		Required:    false,
		Description: "How Discord reaches your server (required for production)",
		ConfigKey:   "tunnel.provider or discord.public_url",
		EnvVar:      envTunnelProvider,
	}

	// If public URL is already set, we're good
	if publicURL != "" {
		check.Status = PrereqOK
		check.Value = publicURL
		return check
	}

	// Check if tunnel is configured
	if cfg.Provider != "" {
		if cfg.Provider == "ngrok" && cfg.NgrokAuthToken == "" {
			check.Status = PrereqInvalid
			check.Value = "ngrok (missing auth token)"
			check.HowToFix = "Provide ngrok authentication token"
			check.Example = fmt.Sprintf(`# Option 1: Add to discord.yaml
tunnel:
  provider: "ngrok"
  ngrok_auth_token: "YOUR_NGROK_TOKEN"

# Option 2: Set environment variable
export %s="YOUR_NGROK_TOKEN"

# Get your token at: https://dashboard.ngrok.com/get-started/your-authtoken`, envNgrokAuthToken)
			return check
		}

		check.Status = PrereqOK
		check.Value = fmt.Sprintf("%s tunnel", cfg.Provider)
		return check
	}

	// No public URL and no tunnel configured
	check.Status = PrereqMissing
	check.Value = "(not configured)"
	check.HowToFix = "Configure a tunnel for development or set a public URL for production"
	check.Example = `# For local development, use a tunnel:
arc-discord server start --tunnel ngrok --ngrok-auth-token $NGROK_TOKEN

# Or use localtunnel (no auth required):
arc-discord server start --tunnel localtunnel

# Or auto-detect available tunnel:
arc-discord server start --tunnel auto

# For production, set your public URL:
discord:
  public_url: "https://your-domain.com/interactions"

# Then configure Discord to send interactions to:
# https://your-domain.com/interactions`
	return check
}

func (c *ServerPrereqChecker) checkApplicationID(appID string) PrereqCheck {
	check := PrereqCheck{
		Name:        "Application ID",
		Required:    true,
		Description: "Your Discord application ID (for responding to interactions)",
		ConfigKey:   "discord.application_id",
	}

	if appID == "" {
		check.Status = PrereqMissing
		check.HowToFix = "Add your Discord application ID"
		check.Example = `# Add to discord.yaml:
discord:
  application_id: "YOUR_APPLICATION_ID"

# Find your Application ID at:
# https://discord.com/developers/applications
# Click your app -> General Information -> APPLICATION ID`
		return check
	}

	check.Status = PrereqOK
	check.Value = appID
	return check
}

// FormatReport formats the prerequisite report for display.
func (r *PrereqReport) FormatReport() string {
	var sb strings.Builder

	sb.WriteString("Discord Server Prerequisites\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	if r.ConfigPath != "" {
		sb.WriteString(fmt.Sprintf("Config: %s\n\n", r.ConfigPath))
	}

	// Group checks by status
	var passed, failed []PrereqCheck
	for _, check := range r.Checks {
		if check.Status == PrereqOK {
			passed = append(passed, check)
		} else {
			failed = append(failed, check)
		}
	}

	// Show passed checks briefly
	if len(passed) > 0 {
		sb.WriteString("Passed:\n")
		for _, check := range passed {
			sb.WriteString(fmt.Sprintf("  [OK] %s", check.Name))
			if check.Value != "" {
				sb.WriteString(fmt.Sprintf(": %s", check.Value))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Show failed checks in detail
	if len(failed) > 0 {
		sb.WriteString("Issues Found:\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n\n")

		for i, check := range failed {
			requiredLabel := ""
			if check.Required {
				requiredLabel = " (REQUIRED)"
			}

			sb.WriteString(fmt.Sprintf("Step %d: %s%s\n", i+1, check.Name, requiredLabel))
			sb.WriteString(fmt.Sprintf("Status: %s\n", check.Status))

			if check.Value != "" {
				sb.WriteString(fmt.Sprintf("Current: %s\n", check.Value))
			}

			sb.WriteString(fmt.Sprintf("\n%s\n", check.Description))

			if check.HowToFix != "" {
				sb.WriteString(fmt.Sprintf("\nHow to fix:\n%s\n", check.HowToFix))
			}

			if check.ConfigKey != "" || check.EnvVar != "" {
				sb.WriteString("\nConfiguration:\n")
				if check.ConfigKey != "" {
					sb.WriteString(fmt.Sprintf("  Config key: %s\n", check.ConfigKey))
				}
				if check.EnvVar != "" {
					sb.WriteString(fmt.Sprintf("  Env var:    %s\n", check.EnvVar))
				}
			}

			if check.Example != "" {
				sb.WriteString(fmt.Sprintf("\nExample:\n%s\n", indentExample(check.Example)))
			}

			sb.WriteString("\n" + strings.Repeat("-", 50) + "\n\n")
		}
	}

	// Summary
	if r.AllPassed {
		sb.WriteString("All prerequisites passed. Ready to start server.\n")
	} else {
		sb.WriteString(fmt.Sprintf("Found %d issue(s) that need to be resolved.\n", len(failed)))
		sb.WriteString("Fix the issues above and try again.\n")
	}

	return sb.String()
}

// FormatQuickFix returns a concise summary of what needs to be fixed.
func (r *PrereqReport) FormatQuickFix() string {
	var sb strings.Builder
	var issues []string

	for _, check := range r.Checks {
		if check.Status != PrereqOK && check.Required {
			issues = append(issues, fmt.Sprintf("- %s: %s", check.Name, check.HowToFix))
		}
	}

	if len(issues) == 0 {
		return ""
	}

	sb.WriteString("Prerequisites not met:\n\n")
	sb.WriteString(strings.Join(issues, "\n"))
	sb.WriteString("\n\nRun with --check-prereqs for detailed setup instructions.\n")

	return sb.String()
}

func indentExample(example string) string {
	lines := strings.Split(example, "\n")
	var indented []string
	for _, line := range lines {
		indented = append(indented, "  "+line)
	}
	return strings.Join(indented, "\n")
}

func parseRedisHost(addr string) string {
	// Extract host from addr like "127.0.0.1:6379"
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// GenerateExampleConfig returns a complete example configuration.
func GenerateExampleConfig() string {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "vibe", "discord.yaml")

	return fmt.Sprintf(`# Discord Server Configuration
# Save this file to: %s

discord:
  # Required: Your Discord bot token
  bot_token: "YOUR_BOT_TOKEN"

  # Required: Your Discord application ID
  # Find at: https://discord.com/developers/applications -> General Information
  application_id: "YOUR_APPLICATION_ID"

  # Required: Your Discord application public key
  # Find at: https://discord.com/developers/applications -> General Information
  public_key: "YOUR_PUBLIC_KEY"

  # Optional: Default guild for slash command registration
  default_guild_id: "YOUR_GUILD_ID"

  # Optional: Default channel for messages
  default_channel_id: "YOUR_CHANNEL_ID"

  # Optional: Webhook URLs
  webhooks:
    default: "https://discord.com/api/webhooks/..."

# HTTP server settings
server:
  listen_addr: "127.0.0.1:8080"

# Redis settings (for pub/sub to agents)
redis:
  addr: "127.0.0.1:6379"
  # password: ""
  # db: 0
  channel_prefix: "arc:discord"

# Tunnel settings (for local development)
tunnel:
  # provider: "ngrok"  # or "localtunnel" or "auto"
  # ngrok_auth_token: "YOUR_NGROK_TOKEN"

# Interaction handlers
interactions:
  enabled: true
  timeout: 30s

  handlers:
    # Slash command handlers
    commands:
      ping:
        agent: "default"
        description: "Ping the bot"

      ask:
        agent: "claude"
        description: "Ask a question"

    # Button/select menu handlers
    components:
      approve_btn:
        agent: "reviewer"

    # Modal submit handlers
    modals:
      feedback_modal:
        agent: "feedback"
`, configPath)
}
