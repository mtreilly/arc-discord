package interactions

import (
	"fmt"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

const interactionResponseFlagEphemeral = 1 << 6

// ResponseBuilder provides a fluent API for constructing interaction responses.
type ResponseBuilder struct {
	resp *types.InteractionResponse
	err  error
}

// NewMessageResponse creates a builder for an immediate channel message response.
func NewMessageResponse(content string) *ResponseBuilder {
	return &ResponseBuilder{
		resp: &types.InteractionResponse{
			Type: types.InteractionResponseChannelMessageWithSource,
			Data: &types.InteractionApplicationCommandCallbackData{
				Content: content,
			},
		},
	}
}

// NewDeferredResponse creates a builder for a deferred message response (ACK).
func NewDeferredResponse() *ResponseBuilder {
	return &ResponseBuilder{
		resp: &types.InteractionResponse{
			Type: types.InteractionResponseDeferredChannelMessageWithSource,
			Data: &types.InteractionApplicationCommandCallbackData{},
		},
	}
}

// NewModalResponse creates a builder for a modal response.
func NewModalResponse(customID, title string) *ResponseBuilder {
	return &ResponseBuilder{
		resp: &types.InteractionResponse{
			Type: types.InteractionResponseModal,
			Data: &types.InteractionApplicationCommandCallbackData{
				CustomID: customID,
				Title:    title,
			},
		},
	}
}

// SetContent updates the message content (message responses only).
func (b *ResponseBuilder) SetContent(content string) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.Content = content
	}
	return b
}

// SetTTS toggles text-to-speech for the response.
func (b *ResponseBuilder) SetTTS(tts bool) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.TTS = tts
	}
	return b
}

// SetAllowedMentions configures allowed mentions.
func (b *ResponseBuilder) SetAllowedMentions(mentions *types.AllowedMentions) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.AllowedMentions = mentions
	}
	return b
}

// AddEmbed appends an embed to the response.
func (b *ResponseBuilder) AddEmbed(embed types.Embed) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.Embeds = append(data.Embeds, embed)
	}
	return b
}

// AddAttachment appends an attachment reference to the response.
func (b *ResponseBuilder) AddAttachment(attachment types.Attachment) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.Attachments = append(data.Attachments, attachment)
	}
	return b
}

// AddComponentRow appends a top-level action row.
func (b *ResponseBuilder) AddComponentRow(row types.Component) *ResponseBuilder {
	return b.AddComponentRows(row)
}

// AddComponentRows appends multiple typed action rows.
func (b *ResponseBuilder) AddComponentRows(rows ...types.Component) *ResponseBuilder {
	for _, comp := range rows {
		if comp == nil {
			b.err = fmt.Errorf("component is nil")
			return b
		}
		mc, err := comp.ToMessageComponent()
		if err != nil {
			b.err = err
			return b
		}
		if mc.Type != types.ComponentTypeActionRow {
			b.err = fmt.Errorf("components[%d].type: only action rows are allowed at the top level", len(b.ensureData().Components))
			return b
		}
		if data := b.ensureData(); data != nil {
			data.Components = append(data.Components, mc)
		}
	}
	return b
}

// SetComponents replaces the component rows.
func (b *ResponseBuilder) SetComponents(rows ...types.Component) *ResponseBuilder {
	data := b.ensureData()
	if data == nil {
		return b
	}
	data.Components = data.Components[:0]
	return b.AddComponentRows(rows...)
}

// SetRawComponents allows setting raw message components (transition helper).
func (b *ResponseBuilder) SetRawComponents(rows ...types.MessageComponent) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		data.Components = rows
	}
	return b
}

// SetModalComponents replaces modal components (modal responses only).
func (b *ResponseBuilder) SetModalComponents(rows ...types.Component) *ResponseBuilder {
	if !b.ensureResponseType(types.InteractionResponseModal) {
		return b
	}
	data := b.ensureData()
	if data == nil {
		return b
	}
	data.Components = data.Components[:0]
	for _, comp := range rows {
		if comp == nil {
			b.err = fmt.Errorf("component is nil")
			return b
		}
		if comp.ComponentType() != types.ComponentTypeActionRow {
			b.err = fmt.Errorf("modal components must be action rows containing text inputs")
			return b
		}
		mc, err := comp.ToMessageComponent()
		if err != nil {
			b.err = err
			return b
		}
		data.Components = append(data.Components, mc)
	}
	return b
}

// SetEphemeral marks the response as ephemeral.
func (b *ResponseBuilder) SetEphemeral(ephemeral bool) *ResponseBuilder {
	if data := b.ensureData(); data != nil {
		if ephemeral {
			data.Flags |= interactionResponseFlagEphemeral
		} else {
			data.Flags &^= interactionResponseFlagEphemeral
		}
	}
	return b
}

// Build validates and returns the interaction response.
func (b *ResponseBuilder) Build() (*types.InteractionResponse, error) {
	if b == nil || b.resp == nil {
		return nil, fmt.Errorf("response builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := b.resp.Validate(); err != nil {
		return nil, err
	}
	return b.resp, nil
}

func (b *ResponseBuilder) ensureData() *types.InteractionApplicationCommandCallbackData {
	if b == nil || b.resp == nil {
		return nil
	}
	if b.resp.Data == nil {
		b.resp.Data = &types.InteractionApplicationCommandCallbackData{}
	}
	return b.resp.Data
}

func (b *ResponseBuilder) ensureResponseType(expected types.InteractionResponseType) bool {
	if b == nil || b.resp == nil {
		return false
	}
	if b.resp.Type != expected {
		b.err = fmt.Errorf("response type mismatch: expected %d got %d", expected, b.resp.Type)
		return false
	}
	return true
}
