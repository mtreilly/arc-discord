package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/discord/webhook"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

func webhookCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Send and inspect Discord webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(webhookSendCmd(opts))
	cmd.AddCommand(webhookListCmd(opts))
	cmd.AddCommand(webhookThreadCmd(opts))

	return cmd
}

func webhookSendCmd(opts *globalOptions) *cobra.Command {
	var (
		namedWebhook     string
		payloadPath      string
		username         string
		avatarURL        string
		threadID         string
		threadName       string
		contentFlag      string
		embedFiles       []string
		componentFiles   []string
		fileSpecs        []string
		spoilerFileSpecs []string
	)

	cmd := &cobra.Command{
		Use:   "send [content]",
		Short: "Send a JSON payload or text message via webhook",
		Args:  cobra.RangeArgs(0, 1),
		Long: `Send messages through configured Discord webhooks. Supports simple text messages or complex payloads
with embeds, files, and components. Webhooks are configured in discord.yaml or passed via --webhook-url.

Configuration in discord.yaml:
  discord:
    webhooks:
      default: "https://discord.com/api/webhooks/..."
      alerts: "https://discord.com/api/webhooks/..."

Payload Structure (types.WebhookMessage):
  {
    "content": "Message text",
    "username": "Bot Name",
    "avatar_url": "https://...",
    "embeds": [{...}],
    "components": [{...}],
    "thread_id": "123456...",
    "thread_name": "New Thread"
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := contentFlag
			if len(args) > 0 {
				content = args[0]
			}
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runWebhookSend(cmd, opts, webhookSendInput{
				webhookName:      namedWebhook,
				payloadPath:      payloadPath,
				content:          content,
				username:         username,
				avatarURL:        avatarURL,
				threadID:         threadID,
				threadName:       threadName,
				embedPaths:       embedFiles,
				componentPaths:   componentFiles,
				fileSpecs:        fileSpecs,
				spoilerFileSpecs: spoilerFileSpecs,
				output:           opts.output,
			})
		},
		Example: `Example:
  # Send a quick message using the default webhook profile
  arc-discord webhook send "Deploy complete"

Example:
  # Use a named webhook entry and inline content flag
  arc-discord webhook send --webhook alerts --content "Build failed"

Example:
  # Post a structured embed defined in JSON
  arc-discord webhook send --payload payload.json

Example:
  # Override username/avatar for branded alerts
  arc-discord webhook send --content "Alert" --username "SecurityBot" --avatar "https://..."

Example:
  # Launch a forum thread and seed it via webhook
  arc-discord webhook send --content "Topic" --thread-name "Deployment #42"

Example:
  # Attach artifacts or logs to the webhook message
  arc-discord webhook send --payload msg.json --file "/path/to/file.log:build.log"`,
	}

	cmd.Flags().StringVar(&namedWebhook, "webhook", "default", "Name of webhook entry from discord.yaml")
	cmd.Flags().StringVar(&payloadPath, "payload", "", "Path to JSON file describing types.WebhookMessage")
	cmd.Flags().StringVar(&username, "username", "", "Override the webhook username")
	cmd.Flags().StringVar(&avatarURL, "avatar", "", "Override the webhook avatar URL")
	cmd.Flags().StringVar(&threadID, "thread-id", "", "Target a specific thread ID")
	cmd.Flags().StringVar(&threadName, "thread-name", "", "Create a new thread with this name (forum channels only)")
	cmd.Flags().StringVar(&contentFlag, "content", "", "Message content when not using positional arg")
	cmd.Flags().StringArrayVar(&embedFiles, "embed-file", nil, "Load embed JSON definition from file (repeatable)")
	cmd.Flags().StringArrayVar(&componentFiles, "component-file", nil, "Load message components JSON definition from file (repeatable)")
	cmd.Flags().StringArrayVar(&fileSpecs, "file", nil, "Attach local file using path[:name]")
	cmd.Flags().StringArrayVar(&spoilerFileSpecs, "spoiler-file", nil, "Attach local file marked as spoiler using path[:name]")

	return cmd
}

func webhookListCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show configured webhook targets",
		Long: `Display all webhooks configured in discord.yaml. URLs are masked for security.
Webhooks are configured under discord.webhooks in the configuration file.

To add a webhook:
  1. In Discord, go to channel settings → Integrations → Webhooks
  2. Click "New Webhook" and copy the URL
  3. Add to discord.yaml under discord.webhooks section`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.output.Resolve(); err != nil {
				return err
			}
			return runWebhookList(cmd, opts, opts.output)
		},
		Example: `Example:
  # Review all configured webhooks in a table
  arc-discord webhook list

Example:
  # Output JSON for auditing or diffing
  arc-discord webhook list --output json

Example:
  # Confirm a specific webhook exists before using it
  arc-discord webhook list | grep alerts

Example:
  # Show YAML to inspect additional metadata
  arc-discord webhook list --output yaml`,
	}
}

type webhookSendInput struct {
	webhookName      string
	payloadPath      string
	content          string
	username         string
	avatarURL        string
	threadID         string
	threadName       string
	embedPaths       []string
	componentPaths   []string
	fileSpecs        []string
	spoilerFileSpecs []string
	output           output.OutputOptions
}

func runWebhookSend(cmd *cobra.Command, opts *globalOptions, in webhookSendInput) error {
	cfg, path, err := opts.loadConfig()
	if err != nil {
		return err
	}
	webhookURL, err := resolveWebhookURL(cfg, opts, in.webhookName)
	if err != nil {
		return &arcer.CLIError{Msg: err.Error(), Hint: "use --webhook-url or add entries under discord.webhooks"}
	}

	msg, err := buildWebhookMessage(in, path)
	if err != nil {
		return err
	}

	dispatcher, err := newWebhookClientFn(cfg, webhookURL)
	if err != nil {
		return (&arcer.CLIError{Msg: fmt.Sprintf("failed to create webhook client for %s", maskWebhookURL(webhookURL))}).WithCause(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	attachmentSpecs, err := collectAttachmentSpecs(in.fileSpecs, in.spoilerFileSpecs)
	if err != nil {
		return err
	}

	if len(attachmentSpecs) > 0 {
		files, cleanup, err := prepareAttachments(attachmentSpecs)
		if err != nil {
			return err
		}
		defer cleanup()

		if err := dispatcher.SendWithFiles(ctx, msg, files); err != nil {
			return (&arcer.CLIError{Msg: "webhook send with files failed"}).WithCause(err)
		}
	} else {
		if err := dispatcher.Send(ctx, msg); err != nil {
			return (&arcer.CLIError{Msg: "webhook send failed"}).WithCause(err)
		}
	}

	result := map[string]string{
		"webhook":     in.webhookName,
		"webhook_url": maskWebhookURL(webhookURL),
		"thread_id":   msg.ThreadID,
		"thread_name": msg.ThreadName,
		"status":      "sent",
	}

	tbl := keyValueTable(result)
	return renderOutput(cmd, in.output, result, tbl)
}

func buildWebhookMessage(in webhookSendInput, _ string) (*types.WebhookMessage, error) {
	if in.payloadPath != "" {
		data, err := os.ReadFile(in.payloadPath)
		if err != nil {
			return nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to read payload %s", in.payloadPath)}).WithCause(err)
		}
		var msg types.WebhookMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, (&arcer.CLIError{Msg: "payload must be valid JSON for types.WebhookMessage"}).WithCause(err)
		}
		if in.threadID != "" {
			msg.ThreadID = in.threadID
		}
		if in.threadName != "" {
			msg.ThreadName = in.threadName
		}
		if in.username != "" {
			msg.Username = in.username
		}
		if in.avatarURL != "" {
			msg.AvatarURL = in.avatarURL
		}
		if len(in.embedPaths) > 0 {
			embeds, err := loadEmbeds(in.embedPaths)
			if err != nil {
				return nil, err
			}
			msg.Embeds = append(msg.Embeds, embeds...)
		}
		if len(in.componentPaths) > 0 {
			comps, err := loadComponents(in.componentPaths)
			if err != nil {
				return nil, err
			}
			msg.Components = append(msg.Components, comps...)
		}
		return &msg, nil
	}

	if in.content == "" {
		return nil, &arcer.CLIError{Msg: "provide message content via argument, --content, or --payload"}
	}

	msg := &types.WebhookMessage{
		Content:    in.content,
		Username:   in.username,
		AvatarURL:  in.avatarURL,
		ThreadID:   in.threadID,
		ThreadName: in.threadName,
	}
	if len(in.embedPaths) > 0 {
		embeds, err := loadEmbeds(in.embedPaths)
		if err != nil {
			return nil, err
		}
		msg.Embeds = embeds
	}
	if len(in.componentPaths) > 0 {
		comps, err := loadComponents(in.componentPaths)
		if err != nil {
			return nil, err
		}
		msg.Components = comps
	}
	return msg, nil
}

func runWebhookList(cmd *cobra.Command, opts *globalOptions, output output.OutputOptions) error {
	cfg, _, err := opts.loadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Discord.Webhooks) == 0 {
		return &arcer.CLIError{Msg: "no webhooks configured", Hint: "add entries under discord.webhooks or set DISCORD_WEBHOOK"}
	}

	type info struct {
		Name string `json:"name" yaml:"name"`
		URL  string `json:"url" yaml:"url"`
		Note string `json:"note" yaml:"note"`
	}
	rows := make([][]string, 0, len(cfg.Discord.Webhooks))
	payload := make([]info, 0, len(cfg.Discord.Webhooks))
	for name, url := range cfg.Discord.Webhooks {
		masked := maskWebhookURL(url)
		payload = append(payload, info{Name: name, URL: masked, Note: "loaded from config"})
		rows = append(rows, []string{name, masked})
	}

	tbl := &tableData{headers: []string{"Name", "Webhook"}, rows: rows}
	return renderOutput(cmd, output, payload, tbl)
}

func maskWebhookURL(raw string) string {
	if raw == "" {
		return ""
	}
	if len(raw) <= 8 {
		return raw
	}
	if len(raw) > 16 {
		return fmt.Sprintf("%s…%s", raw[:12], raw[len(raw)-4:])
	}
	return raw[:len(raw)/2] + "…"
}

type attachmentSpec struct {
	path    string
	name    string
	spoiler bool
}

func collectAttachmentSpecs(regular, spoiler []string) ([]attachmentSpec, error) {
	var specs []attachmentSpec
	for _, raw := range regular {
		spec, err := parseAttachmentSpec(raw, false)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	for _, raw := range spoiler {
		spec, err := parseAttachmentSpec(raw, true)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func parseAttachmentSpec(raw string, spoiler bool) (attachmentSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return attachmentSpec{}, &arcer.CLIError{Msg: "file path cannot be empty"}
	}
	spec := attachmentSpec{path: raw, spoiler: spoiler}
	if idx := strings.Index(raw, ":"); idx != -1 {
		spec.path = raw[:idx]
		spec.name = raw[idx+1:]
	}
	if spec.name == "" {
		spec.name = filepath.Base(spec.path)
	}
	if spoiler && !strings.HasPrefix(spec.name, "SPOILER_") {
		spec.name = "SPOILER_" + spec.name
	}
	return spec, nil
}

func prepareAttachments(specs []attachmentSpec) ([]webhook.FileAttachment, func(), error) {
	files := make([]*os.File, 0, len(specs))
	attachments := make([]webhook.FileAttachment, 0, len(specs))

	for _, spec := range specs {
		f, err := os.Open(spec.path)
		if err != nil {
			cleanupFiles(files)
			return nil, nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to open %s", spec.path)}).WithCause(err)
		}
		info, err := f.Stat()
		if err != nil {
			cleanupFiles(files)
			f.Close()
			return nil, nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to stat %s", spec.path)}).WithCause(err)
		}
		files = append(files, f)

		attachments = append(attachments, webhook.FileAttachment{
			Name:        spec.name,
			ContentType: detectContentType(spec.name),
			Reader:      f,
			Size:        info.Size(),
		})
	}

	cleanup := func() { cleanupFiles(files) }
	return attachments, cleanup, nil
}

func cleanupFiles(files []*os.File) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}

func detectContentType(name string) string {
	if ext := filepath.Ext(name); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	return "application/octet-stream"
}

func loadEmbeds(paths []string) ([]types.Embed, error) {
	var embeds []types.Embed
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to read embed file %s", path)}).WithCause(err)
		}

		if decoded, err := decodeEmbeds(data); err == nil {
			embeds = append(embeds, decoded...)
			continue
		} else {
			return nil, err
		}
	}
	return embeds, nil
}

func decodeEmbeds(data []byte) ([]types.Embed, error) {
	var slice []types.Embed
	if err := json.Unmarshal(data, &slice); err == nil && len(slice) > 0 {
		return slice, nil
	}
	var single types.Embed
	if err := json.Unmarshal(data, &single); err == nil {
		return []types.Embed{single}, nil
	}
	return nil, &arcer.CLIError{Msg: "embed file must contain an embed object or array"}
}

func loadComponents(paths []string) ([]types.MessageComponent, error) {
	var comps []types.MessageComponent
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, (&arcer.CLIError{Msg: fmt.Sprintf("failed to read component file %s", path)}).WithCause(err)
		}
		if decoded, err := decodeComponents(data); err == nil {
			comps = append(comps, decoded...)
			continue
		} else {
			return nil, err
		}
	}
	return comps, nil
}

func decodeComponents(data []byte) ([]types.MessageComponent, error) {
	var slice []types.MessageComponent
	if err := json.Unmarshal(data, &slice); err == nil && len(slice) > 0 {
		return slice, nil
	}
	var single types.MessageComponent
	if err := json.Unmarshal(data, &single); err == nil && single.Type != 0 {
		return []types.MessageComponent{single}, nil
	}
	return nil, &arcer.CLIError{Msg: "component file must contain a message component object or array"}
}
