package interactions

import (
	"fmt"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// ButtonBuilder provides a fluent API for constructing typed buttons.
type ButtonBuilder struct {
	button *types.Button
	err    error
}

// NewButton creates a non-link button builder.
func NewButton(customID, label string, style types.ButtonStyle) *ButtonBuilder {
	return &ButtonBuilder{
		button: &types.Button{
			Style:    style,
			Label:    label,
			CustomID: customID,
		},
	}
}

// NewLinkButton creates a link button builder.
func NewLinkButton(label, url string) *ButtonBuilder {
	return &ButtonBuilder{
		button: &types.Button{
			Style: types.ButtonStyleLink,
			Label: label,
			URL:   url,
		},
	}
}

// SetEmoji sets the button emoji.
func (b *ButtonBuilder) SetEmoji(emoji *types.Emoji) *ButtonBuilder {
	if b.button != nil {
		b.button.Emoji = emoji
	}
	return b
}

// SetDisabled toggles the button disabled flag.
func (b *ButtonBuilder) SetDisabled(disabled bool) *ButtonBuilder {
	if b.button != nil {
		b.button.Disabled = disabled
	}
	return b
}

// Build validates and returns the typed button.
func (b *ButtonBuilder) Build() (*types.Button, error) {
	if b == nil || b.button == nil {
		return nil, fmt.Errorf("button builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.button.Validate(); err != nil {
		return nil, err
	}
	return b.button, nil
}

// SelectMenuBuilder builds select menus.
type SelectMenuBuilder struct {
	menu *types.SelectMenu
	err  error
}

// NewSelectMenu creates a new string select menu builder.
func NewSelectMenu(customID string) *SelectMenuBuilder {
	return &SelectMenuBuilder{
		menu: &types.SelectMenu{
			Type:      types.ComponentTypeSelectMenu,
			CustomID:  customID,
			MinValues: 1,
			MaxValues: 1,
		},
	}
}

// SelectMenuOfType creates a select menu builder for the specified type (user/role/channel/mentionable).
func SelectMenuOfType(customID string, componentType types.ComponentType) *SelectMenuBuilder {
	return &SelectMenuBuilder{
		menu: &types.SelectMenu{
			Type:     componentType,
			CustomID: customID,
		},
	}
}

// AddOption appends an option to the menu (string selects only).
func (b *SelectMenuBuilder) AddOption(label, value, description string, emoji *types.Emoji, def bool) *SelectMenuBuilder {
	if b.menu == nil {
		return b
	}
	if b.menu.ComponentType() != types.ComponentTypeSelectMenu {
		b.err = fmt.Errorf("only string select menus accept options")
		return b
	}
	b.menu.Options = append(b.menu.Options, types.SelectOption{
		Label:       label,
		Value:       value,
		Description: description,
		Emoji:       emoji,
		Default:     def,
	})
	return b
}

// SetPlaceholder sets the select placeholder text.
func (b *SelectMenuBuilder) SetPlaceholder(placeholder string) *SelectMenuBuilder {
	if b.menu != nil {
		b.menu.Placeholder = placeholder
	}
	return b
}

// SetMinMaxValues sets allowed selection counts.
func (b *SelectMenuBuilder) SetMinMaxValues(min, max int) *SelectMenuBuilder {
	if b.menu != nil {
		b.menu.MinValues = min
		b.menu.MaxValues = max
	}
	return b
}

// SetChannelTypes restricts channel select menus to specific channel types.
func (b *SelectMenuBuilder) SetChannelTypes(channelTypes ...types.ChannelType) *SelectMenuBuilder {
	if b.menu != nil {
		b.menu.ChannelTypes = channelTypes
	}
	return b
}

// SetDisabled toggles disabled state.
func (b *SelectMenuBuilder) SetDisabled(disabled bool) *SelectMenuBuilder {
	if b.menu != nil {
		b.menu.Disabled = disabled
	}
	return b
}

// Build validates and returns the select menu.
func (b *SelectMenuBuilder) Build() (*types.SelectMenu, error) {
	if b == nil || b.menu == nil {
		return nil, fmt.Errorf("select menu builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.menu.Validate(); err != nil {
		return nil, err
	}
	return b.menu, nil
}

// TextInputBuilder constructs modal text inputs.
type TextInputBuilder struct {
	input *types.TextInput
	err   error
}

// NewTextInput creates a new text input builder.
func NewTextInput(customID, label string, style types.TextInputStyle) *TextInputBuilder {
	return &TextInputBuilder{
		input: &types.TextInput{
			CustomID: customID,
			Label:    label,
			Style:    style,
		},
	}
}

// SetPlaceholder sets the placeholder text.
func (b *TextInputBuilder) SetPlaceholder(placeholder string) *TextInputBuilder {
	if b.input != nil {
		b.input.Placeholder = placeholder
	}
	return b
}

// SetValue sets the default value.
func (b *TextInputBuilder) SetValue(value string) *TextInputBuilder {
	if b.input != nil {
		b.input.Value = value
	}
	return b
}

// SetRequired toggles the required flag.
func (b *TextInputBuilder) SetRequired(required bool) *TextInputBuilder {
	if b.input != nil {
		b.input.Required = required
	}
	return b
}

// SetLength bounds the min/max length.
func (b *TextInputBuilder) SetLength(min, max int) *TextInputBuilder {
	if b.input != nil {
		b.input.MinLength = min
		b.input.MaxLength = max
	}
	return b
}

// Build validates and returns the text input.
func (b *TextInputBuilder) Build() (*types.TextInput, error) {
	if b == nil || b.input == nil {
		return nil, fmt.Errorf("text input builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.input.Validate(); err != nil {
		return nil, err
	}
	return b.input, nil
}

// ActionRowBuilder constructs action rows.
type ActionRowBuilder struct {
	row *types.ActionRow
	err error
}

// NewActionRow creates an empty action row builder.
func NewActionRow() *ActionRowBuilder {
	return &ActionRowBuilder{
		row: &types.ActionRow{},
	}
}

// AddComponent appends a component to the row.
func (b *ActionRowBuilder) AddComponent(component types.Component) *ActionRowBuilder {
	if component == nil {
		b.err = fmt.Errorf("component is nil")
		return b
	}
	b.row.Components = append(b.row.Components, component)
	return b
}

// Build validates and returns the action row.
func (b *ActionRowBuilder) Build() (*types.ActionRow, error) {
	if b == nil || b.row == nil {
		return nil, fmt.Errorf("action row builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.row.Validate(); err != nil {
		return nil, err
	}
	return b.row, nil
}
