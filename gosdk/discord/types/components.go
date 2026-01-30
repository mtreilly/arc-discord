package types

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxActionRowChildren       = 5
	maxSelectOptions           = 25
	maxPlaceholderLength       = 150
	maxButtonLabelLength       = 80
	maxTextInputLabelLength    = 45
	maxTextInputPlaceholderLen = 100
	textInputMinValueMin       = 0
	textInputMaxValueMax       = 4000
)

// Component describes a typed message component that can be converted into the raw MessageComponent representation.
type Component interface {
	ComponentType() ComponentType
	Validate() error
	ToMessageComponent() (MessageComponent, error)
}

// ActionRow represents a top-level component container.
type ActionRow struct {
	Components []Component
}

// ComponentType returns the component type enum.
func (r *ActionRow) ComponentType() ComponentType {
	return ComponentTypeActionRow
}

// Validate ensures the row and its children satisfy Discord constraints.
func (r *ActionRow) Validate() error {
	if r == nil {
		return &ValidationError{Field: "action_row", Message: "action row is required"}
	}
	if len(r.Components) == 0 {
		return &ValidationError{Field: "action_row.components", Message: "action row must contain at least one component"}
	}
	if len(r.Components) > maxActionRowChildren {
		return &ValidationError{Field: "action_row.components", Message: fmt.Sprintf("action row supports at most %d components", maxActionRowChildren)}
	}
	for i, child := range r.Components {
		if child == nil {
			return &ValidationError{Field: fmt.Sprintf("action_row.components[%d]", i), Message: "component is nil"}
		}
		if child.ComponentType() == ComponentTypeActionRow {
			return &ValidationError{Field: fmt.Sprintf("action_row.components[%d].type", i), Message: "nested action rows are not allowed"}
		}
		if err := child.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ToMessageComponent converts the row and children into raw message components.
func (r *ActionRow) ToMessageComponent() (MessageComponent, error) {
	if err := r.Validate(); err != nil {
		return MessageComponent{}, err
	}
	children := make([]MessageComponent, 0, len(r.Components))
	for _, child := range r.Components {
		mc, err := child.ToMessageComponent()
		if err != nil {
			return MessageComponent{}, err
		}
		children = append(children, mc)
	}
	return MessageComponent{
		Type:       ComponentTypeActionRow,
		Components: children,
	}, nil
}

// ButtonStyle enumerates button style options.
type ButtonStyle int

const (
	ButtonStylePrimary ButtonStyle = iota + 1
	ButtonStyleSecondary
	ButtonStyleSuccess
	ButtonStyleDanger
	ButtonStyleLink
)

// Button represents an interactive button component.
type Button struct {
	Style    ButtonStyle `json:"style"`
	Label    string      `json:"label,omitempty"`
	Emoji    *Emoji      `json:"emoji,omitempty"`
	CustomID string      `json:"custom_id,omitempty"`
	URL      string      `json:"url,omitempty"`
	Disabled bool        `json:"disabled,omitempty"`
}

// ComponentType returns the component type enum value.
func (b *Button) ComponentType() ComponentType {
	return ComponentTypeButton
}

// Validate ensures the button satisfies Discord constraints.
func (b *Button) Validate() error {
	if b == nil {
		return &ValidationError{Field: "button", Message: "button is required"}
	}
	if b.Style < ButtonStylePrimary || b.Style > ButtonStyleLink {
		return &ValidationError{Field: "button.style", Message: "invalid button style"}
	}
	if utf8.RuneCountInString(b.Label) > maxButtonLabelLength {
		return &ValidationError{Field: "button.label", Message: fmt.Sprintf("label must be <= %d characters", maxButtonLabelLength)}
	}
	if strings.TrimSpace(b.Label) == "" && b.Emoji == nil {
		return &ValidationError{Field: "button", Message: "button requires a label or emoji"}
	}

	if b.Style == ButtonStyleLink {
		if strings.TrimSpace(b.URL) == "" {
			return &ValidationError{Field: "button.url", Message: "link buttons require a URL"}
		}
		if _, err := url.ParseRequestURI(b.URL); err != nil {
			return &ValidationError{Field: "button.url", Message: "invalid URL"}
		}
		if b.CustomID != "" {
			return &ValidationError{Field: "button.custom_id", Message: "link buttons cannot have a custom_id"}
		}
	} else {
		if strings.TrimSpace(b.CustomID) == "" {
			return &ValidationError{Field: "button.custom_id", Message: "custom_id is required"}
		}
		if b.URL != "" {
			return &ValidationError{Field: "button.url", Message: "non-link buttons cannot have a URL"}
		}
	}

	return nil
}

// ToMessageComponent converts the button into the raw representation.
func (b *Button) ToMessageComponent() (MessageComponent, error) {
	if err := b.Validate(); err != nil {
		return MessageComponent{}, err
	}
	return MessageComponent{
		Type:     ComponentTypeButton,
		Style:    int(b.Style),
		Label:    b.Label,
		Emoji:    b.Emoji,
		CustomID: b.CustomID,
		URL:      b.URL,
		Disabled: b.Disabled,
	}, nil
}

// SelectMenu represents select menu components (string/user/role/channel/mentionable).
type SelectMenu struct {
	Type         ComponentType  `json:"type"`
	CustomID     string         `json:"custom_id"`
	Placeholder  string         `json:"placeholder,omitempty"`
	MinValues    int            `json:"min_values,omitempty"`
	MaxValues    int            `json:"max_values,omitempty"`
	Options      []SelectOption `json:"options,omitempty"`
	ChannelTypes []ChannelType  `json:"channel_types,omitempty"`
	Disabled     bool           `json:"disabled,omitempty"`
}

// ComponentType returns the component type enum value.
func (s *SelectMenu) ComponentType() ComponentType {
	if s == nil || s.Type == 0 {
		return ComponentTypeSelectMenu
	}
	return s.Type
}

// Validate ensures the select menu satisfies Discord constraints.
func (s *SelectMenu) Validate() error {
	if s == nil {
		return &ValidationError{Field: "select_menu", Message: "select menu is required"}
	}
	if strings.TrimSpace(s.CustomID) == "" {
		return &ValidationError{Field: "select_menu.custom_id", Message: "custom_id is required"}
	}
	if utf8.RuneCountInString(s.Placeholder) > maxPlaceholderLength {
		return &ValidationError{Field: "select_menu.placeholder", Message: fmt.Sprintf("placeholder must be <= %d characters", maxPlaceholderLength)}
	}
	if s.MinValues < 0 || s.MinValues > maxSelectOptions {
		return &ValidationError{Field: "select_menu.min_values", Message: "min_values must be between 0 and 25"}
	}
	if s.MaxValues < 1 || s.MaxValues > maxSelectOptions {
		return &ValidationError{Field: "select_menu.max_values", Message: "max_values must be between 1 and 25"}
	}
	if s.MinValues > s.MaxValues {
		return &ValidationError{Field: "select_menu.min_values", Message: "min_values cannot exceed max_values"}
	}

	switch s.ComponentType() {
	case ComponentTypeSelectMenu:
		if len(s.Options) == 0 {
			return &ValidationError{Field: "select_menu.options", Message: "string select menus require at least one option"}
		}
		if len(s.Options) > maxSelectOptions {
			return &ValidationError{Field: "select_menu.options", Message: fmt.Sprintf("no more than %d options are allowed", maxSelectOptions)}
		}
		for i := range s.Options {
			if err := s.Options[i].Validate(); err != nil {
				return fmt.Errorf("select_menu.options[%d]: %w", i, err)
			}
		}
	default:
		if len(s.Options) > 0 {
			return &ValidationError{Field: "select_menu.options", Message: "non-string select menus cannot define static options"}
		}
	}
	return nil
}

// ToMessageComponent converts the select menu to the raw representation.
func (s *SelectMenu) ToMessageComponent() (MessageComponent, error) {
	if err := s.Validate(); err != nil {
		return MessageComponent{}, err
	}
	return MessageComponent{
		Type:         s.ComponentType(),
		CustomID:     s.CustomID,
		Placeholder:  s.Placeholder,
		MinValues:    s.MinValues,
		MaxValues:    s.MaxValues,
		Options:      s.Options,
		ChannelTypes: s.ChannelTypes,
		Disabled:     s.Disabled,
	}, nil
}

// SelectOption represents a select menu option.
type SelectOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Emoji       *Emoji `json:"emoji,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// Validate ensures the option satisfies Discord constraints.
func (o SelectOption) Validate() error {
	if utf8.RuneCountInString(strings.TrimSpace(o.Label)) == 0 {
		return &ValidationError{Field: "select_option.label", Message: "label is required"}
	}
	if utf8.RuneCountInString(o.Label) > 100 {
		return &ValidationError{Field: "select_option.label", Message: "label must be <= 100 characters"}
	}
	if utf8.RuneCountInString(strings.TrimSpace(o.Value)) == 0 {
		return &ValidationError{Field: "select_option.value", Message: "value is required"}
	}
	if utf8.RuneCountInString(o.Value) > 100 {
		return &ValidationError{Field: "select_option.value", Message: "value must be <= 100 characters"}
	}
	if utf8.RuneCountInString(o.Description) > 100 {
		return &ValidationError{Field: "select_option.description", Message: "description must be <= 100 characters"}
	}
	return nil
}

// TextInputStyle enumerates text input styles.
type TextInputStyle int

const (
	TextInputStyleShort     TextInputStyle = 1
	TextInputStyleParagraph TextInputStyle = 2
)

// TextInput represents modal text input components.
type TextInput struct {
	CustomID    string         `json:"custom_id"`
	Label       string         `json:"label"`
	Style       TextInputStyle `json:"style"`
	MinLength   int            `json:"min_length,omitempty"`
	MaxLength   int            `json:"max_length,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Value       string         `json:"value,omitempty"`
}

// ComponentType returns the component type enum value.
func (t *TextInput) ComponentType() ComponentType {
	return ComponentTypeTextInput
}

// Validate ensures the text input satisfies Discord constraints.
func (t *TextInput) Validate() error {
	if t == nil {
		return &ValidationError{Field: "text_input", Message: "text input is required"}
	}
	if strings.TrimSpace(t.CustomID) == "" {
		return &ValidationError{Field: "text_input.custom_id", Message: "custom_id is required"}
	}
	labelLen := utf8.RuneCountInString(strings.TrimSpace(t.Label))
	if labelLen == 0 {
		return &ValidationError{Field: "text_input.label", Message: "label is required"}
	}
	if labelLen > maxTextInputLabelLength {
		return &ValidationError{Field: "text_input.label", Message: fmt.Sprintf("label must be <= %d characters", maxTextInputLabelLength)}
	}
	if t.Style != TextInputStyleShort && t.Style != TextInputStyleParagraph {
		return &ValidationError{Field: "text_input.style", Message: "invalid text input style"}
	}
	if t.MinLength < textInputMinValueMin || t.MinLength > textInputMaxValueMax {
		return &ValidationError{Field: "text_input.min_length", Message: fmt.Sprintf("min_length must be between %d and %d", textInputMinValueMin, textInputMaxValueMax)}
	}
	if t.MaxLength == 0 {
		t.MaxLength = textInputMaxValueMax
	}
	if t.MaxLength < t.MinLength || t.MaxLength > textInputMaxValueMax {
		return &ValidationError{Field: "text_input.max_length", Message: fmt.Sprintf("max_length must be between %d and %d and >= min_length", textInputMinValueMin, textInputMaxValueMax)}
	}
	if utf8.RuneCountInString(t.Placeholder) > maxTextInputPlaceholderLen {
		return &ValidationError{Field: "text_input.placeholder", Message: fmt.Sprintf("placeholder must be <= %d characters", maxTextInputPlaceholderLen)}
	}
	valueLen := utf8.RuneCountInString(t.Value)
	if valueLen > 0 && (valueLen < t.MinLength || valueLen > t.MaxLength) {
		return &ValidationError{Field: "text_input.value", Message: "value must be within the defined min/max length"}
	}
	return nil
}

// ToMessageComponent converts the text input to the raw representation.
func (t *TextInput) ToMessageComponent() (MessageComponent, error) {
	if err := t.Validate(); err != nil {
		return MessageComponent{}, err
	}
	return MessageComponent{
		Type:        ComponentTypeTextInput,
		CustomID:    t.CustomID,
		Label:       t.Label,
		Style:       int(t.Style),
		MinLength:   t.MinLength,
		MaxLength:   t.MaxLength,
		Placeholder: t.Placeholder,
		Required:    t.Required,
		Value:       t.Value,
	}, nil
}
