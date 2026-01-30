package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// ApplicationCommands provides helpers for managing application commands.
type ApplicationCommands struct {
	client        *Client
	applicationID string
}

// ApplicationCommands creates a command management service scoped to an application ID.
func (c *Client) ApplicationCommands(applicationID string) *ApplicationCommands {
	return &ApplicationCommands{client: c, applicationID: applicationID}
}

// GetGlobalApplicationCommands lists global commands for the application.
func (a *ApplicationCommands) GetGlobalApplicationCommands(ctx context.Context) ([]*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}

	var cmds []*types.ApplicationCommand
	if err := a.client.Get(ctx, a.globalPath(""), &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

// CreateGlobalApplicationCommand registers a new global command.
func (a *ApplicationCommands) CreateGlobalApplicationCommand(ctx context.Context, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateCommand(cmd); err != nil {
		return nil, err
	}

	headers := auditHeaders(cmd.AuditLogReason)
	var created types.ApplicationCommand
	if err := a.client.do(ctx, http.MethodPost, a.globalPath(""), cmd, &created, headers); err != nil {
		return nil, err
	}
	return &created, nil
}

// EditGlobalApplicationCommand updates an existing global command.
func (a *ApplicationCommands) EditGlobalApplicationCommand(ctx context.Context, commandID string, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateID("commandID", commandID); err != nil {
		return nil, err
	}
	if err := validateCommand(cmd); err != nil {
		return nil, err
	}

	headers := auditHeaders(cmd.AuditLogReason)
	var updated types.ApplicationCommand
	if err := a.client.do(ctx, http.MethodPatch, a.globalPath(commandID), cmd, &updated, headers); err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeleteGlobalApplicationCommand removes a global command.
func (a *ApplicationCommands) DeleteGlobalApplicationCommand(ctx context.Context, commandID string) error {
	if err := a.ensureApplicationID(); err != nil {
		return err
	}
	if err := validateID("commandID", commandID); err != nil {
		return err
	}
	return a.client.Delete(ctx, a.globalPath(commandID))
}

// GetGuildApplicationCommands lists guild commands for the given guild.
func (a *ApplicationCommands) GetGuildApplicationCommands(ctx context.Context, guildID string) ([]*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}

	var cmds []*types.ApplicationCommand
	if err := a.client.Get(ctx, a.guildPath(guildID, ""), &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

// CreateGuildApplicationCommand registers a new guild-scoped command.
func (a *ApplicationCommands) CreateGuildApplicationCommand(ctx context.Context, guildID string, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := validateCommand(cmd); err != nil {
		return nil, err
	}

	headers := auditHeaders(cmd.AuditLogReason)
	var created types.ApplicationCommand
	if err := a.client.do(ctx, http.MethodPost, a.guildPath(guildID, ""), cmd, &created, headers); err != nil {
		return nil, err
	}
	return &created, nil
}

// EditGuildApplicationCommand updates an existing guild command.
func (a *ApplicationCommands) EditGuildApplicationCommand(ctx context.Context, guildID, commandID string, cmd *types.ApplicationCommand) (*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := validateID("commandID", commandID); err != nil {
		return nil, err
	}
	if err := validateCommand(cmd); err != nil {
		return nil, err
	}

	headers := auditHeaders(cmd.AuditLogReason)
	var updated types.ApplicationCommand
	if err := a.client.do(ctx, http.MethodPatch, a.guildPath(guildID, commandID), cmd, &updated, headers); err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeleteGuildApplicationCommand removes a guild-scoped command.
func (a *ApplicationCommands) DeleteGuildApplicationCommand(ctx context.Context, guildID, commandID string) error {
	if err := a.ensureApplicationID(); err != nil {
		return err
	}
	if err := validateID("guildID", guildID); err != nil {
		return err
	}
	if err := validateID("commandID", commandID); err != nil {
		return err
	}
	return a.client.Delete(ctx, a.guildPath(guildID, commandID))
}

// BulkOverwriteGlobalApplicationCommands replaces all global commands atomically.
func (a *ApplicationCommands) BulkOverwriteGlobalApplicationCommands(ctx context.Context, cmds []*types.ApplicationCommand) ([]*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateCommandSlice(cmds); err != nil {
		return nil, err
	}

	payload := ensureCommandSlice(cmds)
	var created []*types.ApplicationCommand
	if err := a.client.Put(ctx, a.globalPath(""), payload, &created); err != nil {
		return nil, err
	}
	return created, nil
}

// BulkOverwriteGuildApplicationCommands replaces all guild commands for the guild atomically.
func (a *ApplicationCommands) BulkOverwriteGuildApplicationCommands(ctx context.Context, guildID string, cmds []*types.ApplicationCommand) ([]*types.ApplicationCommand, error) {
	if err := a.ensureApplicationID(); err != nil {
		return nil, err
	}
	if err := validateID("guildID", guildID); err != nil {
		return nil, err
	}
	if err := validateCommandSlice(cmds); err != nil {
		return nil, err
	}

	payload := ensureCommandSlice(cmds)
	var created []*types.ApplicationCommand
	if err := a.client.Put(ctx, a.guildPath(guildID, ""), payload, &created); err != nil {
		return nil, err
	}
	return created, nil
}

func (a *ApplicationCommands) ensureApplicationID() error {
	if strings.TrimSpace(a.applicationID) == "" {
		return &types.ValidationError{Field: "applicationID", Message: "application ID is required"}
	}
	return nil
}

func (a *ApplicationCommands) globalPath(commandID string) string {
	path := fmt.Sprintf("/applications/%s/commands", a.applicationID)
	if commandID != "" {
		path += "/" + commandID
	}
	return path
}

func (a *ApplicationCommands) guildPath(guildID, commandID string) string {
	path := fmt.Sprintf("/applications/%s/guilds/%s/commands", a.applicationID, guildID)
	if commandID != "" {
		path += "/" + commandID
	}
	return path
}

func validateCommand(cmd *types.ApplicationCommand) error {
	if cmd == nil {
		return &types.ValidationError{Field: "command", Message: "command is required"}
	}
	return cmd.Validate()
}

func validateCommandSlice(cmds []*types.ApplicationCommand) error {
	for i, cmd := range cmds {
		if err := validateCommand(cmd); err != nil {
			return fmt.Errorf("command[%d]: %w", i, err)
		}
	}
	return nil
}

func ensureCommandSlice(cmds []*types.ApplicationCommand) []*types.ApplicationCommand {
	if cmds == nil {
		return []*types.ApplicationCommand{}
	}
	return cmds
}

func auditHeaders(reason string) http.Header {
	if strings.TrimSpace(reason) == "" {
		return nil
	}
	headers := http.Header{}
	headers.Set("X-Audit-Log-Reason", url.QueryEscape(reason))
	return headers
}
