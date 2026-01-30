package embeds

import (
	"fmt"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

const (
	maxTitleRunes       = 256
	maxDescriptionRunes = 4096
	maxFieldNameRunes   = 256
	maxFieldValueRunes  = 1024
	maxFields           = 25
)

// Builder provides a fluent API for constructing embeds.
type Builder struct {
	embed *types.Embed
	err   error
}

// New returns a fresh Builder.
func New() *Builder {
	return &Builder{embed: &types.Embed{}}
}

// WithEmbed seeds the builder with an existing embed.
func WithEmbed(embed *types.Embed) *Builder {
	if embed == nil {
		return New()
	}
	return &Builder{embed: embed}
}

// SetTitle sets the embed title (<=256 runes).
func (b *Builder) SetTitle(title string) *Builder {
	if b.err != nil {
		return b
	}
	if len([]rune(title)) > maxTitleRunes {
		b.err = fmt.Errorf("title exceeds %d characters", maxTitleRunes)
		return b
	}
	b.embed.Title = title
	return b
}

// SetDescription sets the embed description (<=4096 chars).
func (b *Builder) SetDescription(desc string) *Builder {
	if b.err != nil {
		return b
	}
	if len([]rune(desc)) > maxDescriptionRunes {
		b.err = fmt.Errorf("description exceeds %d characters", maxDescriptionRunes)
		return b
	}
	b.embed.Description = desc
	return b
}

// SetColor sets the embed color.
func (b *Builder) SetColor(color int) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Color = color
	return b
}

// SetURL sets the embed URL.
func (b *Builder) SetURL(url string) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.URL = url
	return b
}

// SetTimestamp sets the embed timestamp.
func (b *Builder) SetTimestamp(t time.Time) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Timestamp = &t
	return b
}

// SetFooter adds footer text/icon.
func (b *Builder) SetFooter(text, iconURL string) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Footer = &types.EmbedFooter{
		Text:    text,
		IconURL: iconURL,
	}
	return b
}

// SetImage sets the embed image URL.
func (b *Builder) SetImage(url string) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Image = &types.EmbedImage{URL: url}
	return b
}

// SetThumbnail sets the embed thumbnail URL.
func (b *Builder) SetThumbnail(url string) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Thumbnail = &types.EmbedImage{URL: url}
	return b
}

// SetAuthor sets the embed author metadata.
func (b *Builder) SetAuthor(name, url, iconURL string) *Builder {
	if b.err != nil {
		return b
	}
	b.embed.Author = &types.EmbedAuthor{
		Name:    name,
		URL:     url,
		IconURL: iconURL,
	}
	return b
}

// AddField adds a field to the embed (max 25).
func (b *Builder) AddField(name, value string, inline bool) *Builder {
	if b.err != nil {
		return b
	}
	if len(b.embed.Fields) >= maxFields {
		b.err = fmt.Errorf("maximum of %d fields exceeded", maxFields)
		return b
	}
	if len([]rune(name)) > maxFieldNameRunes {
		b.err = fmt.Errorf("field name exceeds %d characters", maxFieldNameRunes)
		return b
	}
	if len([]rune(value)) > maxFieldValueRunes {
		b.err = fmt.Errorf("field value exceeds %d characters", maxFieldValueRunes)
		return b
	}
	b.embed.Fields = append(b.embed.Fields, types.EmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	})
	return b
}

// Build returns the configured embed or an error.
func (b *Builder) Build() (*types.Embed, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.embed, nil
}

// Success quickly builds a green success embed.
func Success(title, description string) (*types.Embed, error) {
	return New().SetTitle(title).SetDescription(description).SetColor(0x57F287).Build()
}

// Error builds a red error embed.
func Error(title, description string) (*types.Embed, error) {
	return New().SetTitle(title).SetDescription(description).SetColor(0xED4245).Build()
}
