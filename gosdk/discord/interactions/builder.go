package interactions

import (
	"fmt"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// CommandBuilder builds application commands fluently.
type CommandBuilder struct {
	cmd *types.ApplicationCommand
	err error
}

// NewSlashCommand initializes a chat-input command builder.
func NewSlashCommand(name, description string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &types.ApplicationCommand{
			Name:        name,
			Description: description,
			Type:        types.ApplicationCommandTypeChatInput,
		},
	}
}

// AddStringOption appends a string option to the command.
func (b *CommandBuilder) AddStringOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionString, name, description, required))
}

// AddIntegerOption appends an integer option to the command.
func (b *CommandBuilder) AddIntegerOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionInteger, name, description, required))
}

// AddBooleanOption appends a boolean option to the command.
func (b *CommandBuilder) AddBooleanOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionBoolean, name, description, required))
}

// AddUserOption appends a user option to the command.
func (b *CommandBuilder) AddUserOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionUser, name, description, required))
}

// AddChannelOption appends a channel option, optionally constraining channel types.
func (b *CommandBuilder) AddChannelOption(name, description string, required bool, channelTypes ...types.ChannelType) *CommandBuilder {
	opt := newSimpleOption(types.CommandOptionChannel, name, description, required)
	if len(channelTypes) > 0 {
		opt.ChannelTypes = append([]types.ChannelType{}, channelTypes...)
	}
	return b.addOption(opt)
}

// AddRoleOption appends a role option to the command.
func (b *CommandBuilder) AddRoleOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionRole, name, description, required))
}

// AddMentionableOption appends a mentionable option to the command.
func (b *CommandBuilder) AddMentionableOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionMentionable, name, description, required))
}

// AddNumberOption appends a floating-point option to the command.
func (b *CommandBuilder) AddNumberOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionNumber, name, description, required))
}

// AddAttachmentOption appends an attachment option to the command.
func (b *CommandBuilder) AddAttachmentOption(name, description string, required bool) *CommandBuilder {
	return b.addOption(newSimpleOption(types.CommandOptionAttachment, name, description, required))
}

// AddChoices attaches named choices to an existing option (string/integer/number).
func (b *CommandBuilder) AddChoices(optionName string, choices ...types.ApplicationCommandChoice) *CommandBuilder {
	if !b.ensureMutable() || len(choices) == 0 {
		return b
	}
	if err := appendChoices(b.cmd.Options, optionName, choices); err != nil {
		b.err = err
	}
	return b
}

// AddSubcommand appends a subcommand with optional nested options.
func (b *CommandBuilder) AddSubcommand(name, description string, configure func(*SubcommandBuilder)) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	sub := types.ApplicationCommandOption{
		Type:        types.CommandOptionSubCommand,
		Name:        name,
		Description: description,
	}
	if configure != nil {
		configure(&SubcommandBuilder{option: &sub, err: &b.err})
	}
	return b.addOption(sub)
}

// AddSubcommandGroup appends a subcommand group.
func (b *CommandBuilder) AddSubcommandGroup(name, description string, configure func(*SubcommandGroupBuilder)) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	group := types.ApplicationCommandOption{
		Type:        types.CommandOptionSubCommandGroup,
		Name:        name,
		Description: description,
	}
	if configure != nil {
		configure(&SubcommandGroupBuilder{option: &group, err: &b.err})
	}
	return b.addOption(group)
}

// SetDefaultPermission controls DM permission (legacy default permission behavior).
func (b *CommandBuilder) SetDefaultPermission(allow bool) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	value := allow
	b.cmd.DMPermission = &value
	return b
}

// SetDefaultMemberPermissions sets the required permission bitset (as a string).
func (b *CommandBuilder) SetDefaultMemberPermissions(bitset string) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	if bitset == "" {
		b.cmd.DefaultMemberPermissions = nil
		return b
	}
	copy := bitset
	b.cmd.DefaultMemberPermissions = &copy
	return b
}

// SetNSFW marks the command as NSFW.
func (b *CommandBuilder) SetNSFW(nsfw bool) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	b.cmd.NSFW = nsfw
	return b
}

// Build validates and returns the command.
func (b *CommandBuilder) Build() (*types.ApplicationCommand, error) {
	if b == nil {
		return nil, fmt.Errorf("command builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.cmd.Validate(); err != nil {
		return nil, err
	}
	return b.cmd, nil
}

func (b *CommandBuilder) addOption(opt types.ApplicationCommandOption) *CommandBuilder {
	if !b.ensureMutable() {
		return b
	}
	b.cmd.Options = append(b.cmd.Options, opt)
	return b
}

func (b *CommandBuilder) ensureMutable() bool {
	return b != nil && b.cmd != nil && b.err == nil
}

func newSimpleOption(kind types.ApplicationCommandOptionType, name, description string, required bool) types.ApplicationCommandOption {
	return types.ApplicationCommandOption{
		Type:        kind,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func supportsChoices(kind types.ApplicationCommandOptionType) bool {
	switch kind {
	case types.CommandOptionString, types.CommandOptionInteger, types.CommandOptionNumber:
		return true
	default:
		return false
	}
}

func findOptionByName(options []types.ApplicationCommandOption, name string) *types.ApplicationCommandOption {
	for i := range options {
		if options[i].Name == name {
			return &options[i]
		}
	}
	return nil
}

func appendChoices(options []types.ApplicationCommandOption, name string, choices []types.ApplicationCommandChoice) error {
	if len(choices) == 0 {
		return nil
	}
	opt := findOptionByName(options, name)
	if opt == nil {
		return fmt.Errorf("option %q not found", name)
	}
	if !supportsChoices(opt.Type) {
		return fmt.Errorf("option %q does not support choices", name)
	}
	opt.Choices = append(opt.Choices, choices...)
	return nil
}

// SubcommandBuilder configures options for a subcommand.
type SubcommandBuilder struct {
	option *types.ApplicationCommandOption
	err    *error
}

// AddStringOption appends a string option to the subcommand.
func (sb *SubcommandBuilder) AddStringOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionString, name, description, required))
}

// AddIntegerOption appends an integer option to the subcommand.
func (sb *SubcommandBuilder) AddIntegerOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionInteger, name, description, required))
}

// AddBooleanOption appends a boolean option to the subcommand.
func (sb *SubcommandBuilder) AddBooleanOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionBoolean, name, description, required))
}

// AddUserOption appends a user option to the subcommand.
func (sb *SubcommandBuilder) AddUserOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionUser, name, description, required))
}

// AddChannelOption appends a channel option to the subcommand.
func (sb *SubcommandBuilder) AddChannelOption(name, description string, required bool, channelTypes ...types.ChannelType) *SubcommandBuilder {
	opt := newSimpleOption(types.CommandOptionChannel, name, description, required)
	if len(channelTypes) > 0 {
		opt.ChannelTypes = append([]types.ChannelType{}, channelTypes...)
	}
	return sb.addOption(opt)
}

// AddRoleOption appends a role option to the subcommand.
func (sb *SubcommandBuilder) AddRoleOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionRole, name, description, required))
}

// AddMentionableOption appends a mentionable option to the subcommand.
func (sb *SubcommandBuilder) AddMentionableOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionMentionable, name, description, required))
}

// AddNumberOption appends a number option to the subcommand.
func (sb *SubcommandBuilder) AddNumberOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionNumber, name, description, required))
}

// AddAttachmentOption appends an attachment option to the subcommand.
func (sb *SubcommandBuilder) AddAttachmentOption(name, description string, required bool) *SubcommandBuilder {
	return sb.addOption(newSimpleOption(types.CommandOptionAttachment, name, description, required))
}

// AddChoices attaches choices to a subcommand option.
func (sb *SubcommandBuilder) AddChoices(optionName string, choices ...types.ApplicationCommandChoice) *SubcommandBuilder {
	if sb == nil || sb.option == nil || sb.err == nil || len(choices) == 0 || *sb.err != nil {
		return sb
	}
	if err := appendChoices(sb.option.Options, optionName, choices); err != nil {
		*sb.err = err
	}
	return sb
}

func (sb *SubcommandBuilder) addOption(opt types.ApplicationCommandOption) *SubcommandBuilder {
	if sb == nil || sb.option == nil || sb.err == nil || *sb.err != nil {
		return sb
	}
	sb.option.Options = append(sb.option.Options, opt)
	return sb
}

// SubcommandGroupBuilder configures a subcommand group.
type SubcommandGroupBuilder struct {
	option *types.ApplicationCommandOption
	err    *error
}

// AddSubcommand appends a subcommand to the group.
func (gb *SubcommandGroupBuilder) AddSubcommand(name, description string, configure func(*SubcommandBuilder)) *SubcommandGroupBuilder {
	if gb == nil || gb.option == nil || gb.err == nil || *gb.err != nil {
		return gb
	}
	sub := types.ApplicationCommandOption{
		Type:        types.CommandOptionSubCommand,
		Name:        name,
		Description: description,
	}
	if configure != nil {
		configure(&SubcommandBuilder{option: &sub, err: gb.err})
	}
	gb.option.Options = append(gb.option.Options, sub)
	return gb
}
