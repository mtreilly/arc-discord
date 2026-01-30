package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxInteractionResponseContentLength = 2000
	maxInteractionResponseEmbeds        = 10
	maxInteractionResponseComponents    = 5
	maxInteractionComponentsPerRow      = 5
	maxInteractionResponseAttachments   = 10
	maxAutocompleteChoices              = 25
	modalTitleMinRunes                  = 1
	modalTitleMaxRunes                  = 45
	modalCustomIDMinRunes               = 1
	modalCustomIDMaxRunes               = 100
)

// InteractionType defines the type of interaction from Discord.
type InteractionType int

const (
	InteractionTypePing InteractionType = iota + 1
	InteractionTypeApplicationCommand
	InteractionTypeMessageComponent
	InteractionTypeApplicationCommandAutocomplete
	InteractionTypeModalSubmit
)

// Interaction represents an incoming interaction payload.
type Interaction struct {
	ID            string           `json:"id"`
	ApplicationID string           `json:"application_id"`
	Type          InteractionType  `json:"type"`
	Data          *InteractionData `json:"data,omitempty"`
	GuildID       string           `json:"guild_id,omitempty"`
	ChannelID     string           `json:"channel_id,omitempty"`
	Member        *Member          `json:"member,omitempty"`
	User          *User            `json:"user,omitempty"`
	Token         string           `json:"token"`
	Version       int              `json:"version"`
	Message       *Message         `json:"message,omitempty"`
	Locale        string           `json:"locale,omitempty"`
	GuildLocale   string           `json:"guild_locale,omitempty"`
}

// InteractionData contains payload-specific data (commands/components).
type InteractionData struct {
	ID            string                     `json:"id,omitempty"`
	Name          string                     `json:"name,omitempty"`
	Type          ApplicationCommandType     `json:"type,omitempty"`
	Resolved      *ResolvedData              `json:"resolved,omitempty"`
	Options       []ApplicationCommandOption `json:"options,omitempty"`
	CustomID      string                     `json:"custom_id,omitempty"`
	ComponentType ComponentType              `json:"component_type,omitempty"`
	Values        []string                   `json:"values,omitempty"`
	TargetID      string                     `json:"target_id,omitempty"`
}

// ResolvedData contains hydrated entities referenced in commands.
type ResolvedData struct {
	Users    map[string]User    `json:"users,omitempty"`
	Members  map[string]Member  `json:"members,omitempty"`
	Roles    map[string]Role    `json:"roles,omitempty"`
	Channels map[string]Channel `json:"channels,omitempty"`
	Messages map[string]Message `json:"messages,omitempty"`
}

// ApplicationCommand represents a slash command or user/message command.
type ApplicationCommand struct {
	ID                       string                     `json:"id,omitempty"`
	Type                     ApplicationCommandType     `json:"type,omitempty"`
	ApplicationID            string                     `json:"application_id,omitempty"`
	GuildID                  string                     `json:"guild_id,omitempty"`
	Name                     string                     `json:"name"`
	NameLocalizations        map[string]string          `json:"name_localizations,omitempty"`
	Description              string                     `json:"description"`
	DescriptionLocalizations map[string]string          `json:"description_localizations,omitempty"`
	Options                  []ApplicationCommandOption `json:"options,omitempty"`
	DefaultMemberPermissions *string                    `json:"default_member_permissions,omitempty"`
	DMPermission             *bool                      `json:"dm_permission,omitempty"`
	NSFW                     bool                       `json:"nsfw,omitempty"`
	Version                  string                     `json:"version,omitempty"`
	AuditLogReason           string                     `json:"-"`
}

// ApplicationCommandType enumerates command kinds.
type ApplicationCommandType int

const (
	ApplicationCommandTypeChatInput ApplicationCommandType = iota + 1
	ApplicationCommandTypeUser
	ApplicationCommandTypeMessage
)

// ApplicationCommandOption describes command options or subcommands.
type ApplicationCommandOption struct {
	Type                     ApplicationCommandOptionType `json:"type"`
	Name                     string                       `json:"name"`
	NameLocalizations        map[string]string            `json:"name_localizations,omitempty"`
	Description              string                       `json:"description"`
	DescriptionLocalizations map[string]string            `json:"description_localizations,omitempty"`
	Required                 bool                         `json:"required,omitempty"`
	Choices                  []ApplicationCommandChoice   `json:"choices,omitempty"`
	Options                  []ApplicationCommandOption   `json:"options,omitempty"`
	ChannelTypes             []ChannelType                `json:"channel_types,omitempty"`
	MinValue                 *float64                     `json:"min_value,omitempty"`
	MaxValue                 *float64                     `json:"max_value,omitempty"`
	MinLength                *int                         `json:"min_length,omitempty"`
	MaxLength                *int                         `json:"max_length,omitempty"`
	Autocomplete             bool                         `json:"autocomplete,omitempty"`
}

// ApplicationCommandOptionType enumerates option types.
type ApplicationCommandOptionType int

const (
	CommandOptionSubCommand ApplicationCommandOptionType = iota + 1
	CommandOptionSubCommandGroup
	CommandOptionString
	CommandOptionInteger
	CommandOptionBoolean
	CommandOptionUser
	CommandOptionChannel
	CommandOptionRole
	CommandOptionMentionable
	CommandOptionNumber
	CommandOptionAttachment
)

// ApplicationCommandChoice represents an option choice.
type ApplicationCommandChoice struct {
	Name              string            `json:"name"`
	NameLocalizations map[string]string `json:"name_localizations,omitempty"`
	Value             interface{}       `json:"value"`
}

// ComponentType enumerates message component types.
type ComponentType int

const (
	ComponentTypeActionRow         ComponentType = 1
	ComponentTypeButton            ComponentType = 2
	ComponentTypeStringSelect      ComponentType = 3
	ComponentTypeTextInput         ComponentType = 4
	ComponentTypeUserSelect        ComponentType = 5
	ComponentTypeRoleSelect        ComponentType = 6
	ComponentTypeMentionableSelect ComponentType = 7
	ComponentTypeChannelSelect     ComponentType = 8
)

// ComponentTypeSelectMenu is kept for backwards compatibility with the old naming.
const ComponentTypeSelectMenu ComponentType = ComponentTypeStringSelect

// Validate ensures interactions are well-formed.
func (i *Interaction) Validate() error {
	if i == nil {
		return &ValidationError{Field: "interaction", Message: "interaction is required"}
	}
	if i.ID == "" {
		return &ValidationError{Field: "interaction.id", Message: "interaction ID is required"}
	}
	if i.Token == "" {
		return &ValidationError{Field: "interaction.token", Message: "interaction token is required"}
	}
	return nil
}

// Validate ensures command definitions satisfy Discord constraints.
func (c *ApplicationCommand) Validate() error {
	if c == nil {
		return &ValidationError{Field: "command", Message: "command is required"}
	}
	if len(c.Name) < 1 || len(c.Name) > 32 {
		return &ValidationError{Field: "name", Message: "command name must be 1-32 characters"}
	}
	if len(c.Description) > 100 {
		return &ValidationError{Field: "description", Message: "description must be <=100 characters"}
	}
	for _, opt := range c.Options {
		if err := opt.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures option details respect Discord limits.
func (o *ApplicationCommandOption) Validate() error {
	if len(o.Name) < 1 || len(o.Name) > 32 {
		return &ValidationError{Field: "option.name", Message: "option name must be 1-32 characters"}
	}
	if len(o.Description) < 1 || len(o.Description) > 100 {
		return &ValidationError{Field: "option.description", Message: "description must be 1-100 characters"}
	}
	for _, opt := range o.Options {
		if err := opt.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// InteractionResponseType enumerates response types.
type InteractionResponseType int

const (
	InteractionResponsePong                             InteractionResponseType = 1
	InteractionResponseChannelMessageWithSource         InteractionResponseType = 4
	InteractionResponseDeferredChannelMessageWithSource InteractionResponseType = 5
	InteractionResponseDeferredUpdateMessage            InteractionResponseType = 6
	InteractionResponseUpdateMessage                    InteractionResponseType = 7
	InteractionResponseAutocompleteResult               InteractionResponseType = 8
	InteractionResponseModal                            InteractionResponseType = 9
)

// InteractionResponse represents the payload for responding to interactions.
type InteractionResponse struct {
	Type InteractionResponseType                    `json:"type"`
	Data *InteractionApplicationCommandCallbackData `json:"data,omitempty"`
}

// InteractionApplicationCommandCallbackData describes response data.
type InteractionApplicationCommandCallbackData struct {
	TTS             bool                 `json:"tts,omitempty"`
	Content         string               `json:"content,omitempty"`
	Embeds          []Embed              `json:"embeds,omitempty"`
	AllowedMentions *AllowedMentions     `json:"allowed_mentions,omitempty"`
	Flags           int                  `json:"flags,omitempty"`
	Components      []MessageComponent   `json:"components,omitempty"`
	Attachments     []Attachment         `json:"attachments,omitempty"`
	Choices         []AutocompleteChoice `json:"choices,omitempty"`
	CustomID        string               `json:"custom_id,omitempty"`
	Title           string               `json:"title,omitempty"`
}

// MessageComponent represents a generic component.
type MessageComponent struct {
	Type         ComponentType      `json:"type"`
	CustomID     string             `json:"custom_id,omitempty"`
	Disabled     bool               `json:"disabled,omitempty"`
	Style        int                `json:"style,omitempty"`
	Label        string             `json:"label,omitempty"`
	Emoji        *Emoji             `json:"emoji,omitempty"`
	URL          string             `json:"url,omitempty"`
	Options      []SelectOption     `json:"options,omitempty"`
	Placeholder  string             `json:"placeholder,omitempty"`
	MinValues    int                `json:"min_values,omitempty"`
	MaxValues    int                `json:"max_values,omitempty"`
	ChannelTypes []ChannelType      `json:"channel_types,omitempty"`
	Components   []MessageComponent `json:"components,omitempty"`
	MinLength    int                `json:"min_length,omitempty"`
	MaxLength    int                `json:"max_length,omitempty"`
	Required     bool               `json:"required,omitempty"`
	Value        string             `json:"value,omitempty"`
}

// AllowedMentions controls mention parsing in responses.
type AllowedMentions struct {
	Parse       []string `json:"parse,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Users       []string `json:"users,omitempty"`
	RepliedUser bool     `json:"replied_user,omitempty"`
}

// AutocompleteChoice represents an entry shown during autocomplete interactions.
type AutocompleteChoice struct {
	Name              string            `json:"name"`
	NameLocalizations map[string]string `json:"name_localizations,omitempty"`
	Value             interface{}       `json:"value"`
}

// Validate ensures the interaction response is well formed for its type.
func (r *InteractionResponse) Validate() error {
	if r == nil {
		return &ValidationError{Field: "response", Message: "response is required"}
	}
	if !isSupportedInteractionResponseType(r.Type) {
		return &ValidationError{Field: "response.type", Message: "unsupported interaction response type"}
	}

	if responseTypeRequiresData(r.Type) && r.Data == nil {
		return &ValidationError{Field: "response.data", Message: "response data is required for this response type"}
	}

	if r.Data != nil {
		if err := r.Data.Validate(r.Type); err != nil {
			return err
		}
	}

	return nil
}

// Validate enforces Discord limits for response payloads.
func (d *InteractionApplicationCommandCallbackData) Validate(responseType InteractionResponseType) error {
	if d == nil {
		return &ValidationError{Field: "response.data", Message: "response data is required"}
	}

	if err := validateResponseContent(d.Content); err != nil {
		return err
	}
	if len(d.Embeds) > maxInteractionResponseEmbeds {
		return &ValidationError{Field: "response.data.embeds", Message: fmt.Sprintf("no more than %d embeds are allowed", maxInteractionResponseEmbeds)}
	}
	if len(d.Attachments) > maxInteractionResponseAttachments {
		return &ValidationError{Field: "response.data.attachments", Message: fmt.Sprintf("no more than %d attachments are allowed", maxInteractionResponseAttachments)}
	}

	switch responseType {
	case InteractionResponseAutocompleteResult:
		return d.validateAutocompletePayload()
	case InteractionResponseModal:
		return d.validateModalPayload()
	default:
		if len(d.Choices) > 0 {
			return &ValidationError{Field: "response.data.choices", Message: "choices are only permitted for autocomplete responses"}
		}
		if err := validateComponentLayout(d.Components, false, false, "response.data.components"); err != nil {
			return err
		}
		return nil
	}
}

func (d *InteractionApplicationCommandCallbackData) validateAutocompletePayload() error {
	if d.TTS || d.Content != "" || len(d.Embeds) > 0 || d.AllowedMentions != nil || d.Flags != 0 || len(d.Components) > 0 || len(d.Attachments) > 0 {
		return &ValidationError{Field: "response.data", Message: "autocomplete responses may only include choices"}
	}
	if len(d.Choices) == 0 {
		return &ValidationError{Field: "response.data.choices", Message: "at least one choice is required"}
	}
	if len(d.Choices) > maxAutocompleteChoices {
		return &ValidationError{Field: "response.data.choices", Message: fmt.Sprintf("no more than %d choices are allowed", maxAutocompleteChoices)}
	}
	for i, choice := range d.Choices {
		if err := choice.Validate(); err != nil {
			return fmt.Errorf("response.data.choices[%d]: %w", i, err)
		}
	}
	return nil
}

func (d *InteractionApplicationCommandCallbackData) validateModalPayload() error {
	customID := strings.TrimSpace(d.CustomID)
	if l := utf8.RuneCountInString(customID); l < modalCustomIDMinRunes || l > modalCustomIDMaxRunes {
		return &ValidationError{Field: "response.data.custom_id", Message: "custom_id must be 1-100 characters"}
	}
	title := strings.TrimSpace(d.Title)
	if l := utf8.RuneCountInString(title); l < modalTitleMinRunes || l > modalTitleMaxRunes {
		return &ValidationError{Field: "response.data.title", Message: "title must be 1-45 characters"}
	}
	if len(d.Choices) > 0 {
		return &ValidationError{Field: "response.data.choices", Message: "choices are not valid for modal responses"}
	}
	if d.Content != "" || len(d.Embeds) > 0 || len(d.Attachments) > 0 || d.AllowedMentions != nil || d.TTS || d.Flags != 0 {
		return &ValidationError{Field: "response.data", Message: "modal responses only support custom_id, title, and components"}
	}
	if err := validateComponentLayout(d.Components, true, true, "response.data.components"); err != nil {
		return err
	}
	return nil
}

// Validate ensures autocomplete choices respect Discord limits.
func (c AutocompleteChoice) Validate() error {
	name := strings.TrimSpace(c.Name)
	if count := utf8.RuneCountInString(name); count < 1 || count > 100 {
		return &ValidationError{Field: "choice.name", Message: "name must be 1-100 characters"}
	}
	if c.Value == nil {
		return &ValidationError{Field: "choice.value", Message: "value is required"}
	}

	switch v := c.Value.(type) {
	case string:
		val := strings.TrimSpace(v)
		if count := utf8.RuneCountInString(val); count < 1 || count > 100 {
			return &ValidationError{Field: "choice.value", Message: "string values must be 1-100 characters"}
		}
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		// valid numeric choice
	default:
		return &ValidationError{Field: "choice.value", Message: "value must be a string or number"}
	}

	return nil
}

func isSupportedInteractionResponseType(t InteractionResponseType) bool {
	switch t {
	case InteractionResponsePong,
		InteractionResponseChannelMessageWithSource,
		InteractionResponseDeferredChannelMessageWithSource,
		InteractionResponseDeferredUpdateMessage,
		InteractionResponseUpdateMessage,
		InteractionResponseAutocompleteResult,
		InteractionResponseModal:
		return true
	default:
		return false
	}
}

func responseTypeRequiresData(t InteractionResponseType) bool {
	switch t {
	case InteractionResponseChannelMessageWithSource,
		InteractionResponseUpdateMessage,
		InteractionResponseAutocompleteResult,
		InteractionResponseModal:
		return true
	default:
		return false
	}
}

func validateResponseContent(content string) error {
	if content == "" {
		return nil
	}
	if utf8.RuneCountInString(content) > maxInteractionResponseContentLength {
		return &ValidationError{
			Field:   "response.data.content",
			Message: fmt.Sprintf("content must be <= %d characters", maxInteractionResponseContentLength),
		}
	}
	return nil
}

func validateComponentLayout(components []MessageComponent, allowTextInputs bool, requireComponents bool, field string) error {
	if len(components) == 0 {
		if requireComponents {
			return &ValidationError{Field: field, Message: "at least one component is required"}
		}
		return nil
	}
	if len(components) > maxInteractionResponseComponents {
		return &ValidationError{Field: field, Message: fmt.Sprintf("no more than %d action rows are allowed", maxInteractionResponseComponents)}
	}
	for i, row := range components {
		if row.Type != ComponentTypeActionRow {
			return &ValidationError{Field: fmt.Sprintf("%s[%d].type", field, i), Message: "top-level components must be action rows"}
		}
		if len(row.Components) == 0 {
			return &ValidationError{Field: fmt.Sprintf("%s[%d].components", field, i), Message: "action row must contain at least one component"}
		}
		if allowTextInputs {
			if len(row.Components) != 1 {
				return &ValidationError{Field: fmt.Sprintf("%s[%d].components", field, i), Message: "modal action rows must contain exactly one text input"}
			}
		} else if len(row.Components) > maxInteractionComponentsPerRow {
			return &ValidationError{Field: fmt.Sprintf("%s[%d].components", field, i), Message: fmt.Sprintf("action rows support up to %d components", maxInteractionComponentsPerRow)}
		}

		for j, child := range row.Components {
			if child.Type == ComponentTypeActionRow {
				return &ValidationError{Field: fmt.Sprintf("%s[%d].components[%d].type", field, i, j), Message: "nested action rows are not allowed"}
			}
			if child.Type == ComponentTypeTextInput {
				if !allowTextInputs {
					return &ValidationError{Field: fmt.Sprintf("%s[%d].components[%d].type", field, i, j), Message: "text inputs are only supported in modal responses"}
				}
				if strings.TrimSpace(child.CustomID) == "" {
					return &ValidationError{Field: fmt.Sprintf("%s[%d].components[%d].custom_id", field, i, j), Message: "text inputs require a custom_id"}
				}
				if strings.TrimSpace(child.Label) == "" {
					return &ValidationError{Field: fmt.Sprintf("%s[%d].components[%d].label", field, i, j), Message: "text inputs require a label"}
				}
			} else if allowTextInputs {
				return &ValidationError{Field: fmt.Sprintf("%s[%d].components[%d].type", field, i, j), Message: "modal components must be text inputs"}
			}
		}
	}
	return nil
}
