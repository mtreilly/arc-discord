package types

import "time"

// Message represents a Discord message
type Message struct {
	ID              string          `json:"id"`
	ChannelID       string          `json:"channel_id"`
	GuildID         string          `json:"guild_id,omitempty"`
	Content         string          `json:"content"`
	Timestamp       time.Time       `json:"timestamp"`
	EditedTimestamp *time.Time      `json:"edited_timestamp,omitempty"`
	Author          *User           `json:"author,omitempty"`
	Embeds          []Embed         `json:"embeds,omitempty"`
	Attachments     []Attachment    `json:"attachments,omitempty"`
	Mentions        []User          `json:"mentions,omitempty"`
	Flags           int             `json:"flags,omitempty"`
	Thread          *Channel        `json:"thread,omitempty"`
	ThreadMetadata  *ThreadMetadata `json:"thread_metadata,omitempty"`
}

// User represents a Discord user
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
}

// Embed represents a Discord message embed
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Timestamp   *time.Time   `json:"timestamp,omitempty"`
	Color       int          `json:"color,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Image       *EmbedImage  `json:"image,omitempty"`
	Thumbnail   *EmbedImage  `json:"thumbnail,omitempty"`
	Author      *EmbedAuthor `json:"author,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
}

// EmbedFooter represents an embed footer
type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedImage represents an embed image or thumbnail
type EmbedImage struct {
	URL    string `json:"url"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

// EmbedAuthor represents an embed author
type EmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedField represents an embed field
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Attachment represents a message attachment
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url"`
	Height   int    `json:"height,omitempty"`
	Width    int    `json:"width,omitempty"`
}

// MessageCreateParams represents parameters for creating a message
type MessageCreateParams struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
	// Add more fields as needed (components, attachments, etc.)
}

// MessageEditParams represents editable message fields.
type MessageEditParams struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}
