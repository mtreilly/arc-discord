package types

// WebhookMessage represents a message to be sent via webhook
type WebhookMessage struct {
	Content         string             `json:"content,omitempty"`
	Username        string             `json:"username,omitempty"`
	AvatarURL       string             `json:"avatar_url,omitempty"`
	TTS             bool               `json:"tts,omitempty"`
	Embeds          []Embed            `json:"embeds,omitempty"`
	Components      []MessageComponent `json:"components,omitempty"`
	AllowedMentions *struct {
		Parse []string `json:"parse,omitempty"`
	} `json:"allowed_mentions,omitempty"`

	// Thread support
	// ThreadID sends the message to an existing thread (instead of the channel)
	ThreadID string `json:"-"` // Sent as query parameter, not in JSON body

	// ThreadName creates a new forum thread with this name (forum channels only)
	// Only works when sending to a forum channel, ignored otherwise
	ThreadName string `json:"thread_name,omitempty"`
}

// Validate checks if the webhook message is valid
func (w *WebhookMessage) Validate() error {
	if w.Content == "" && len(w.Embeds) == 0 {
		return &ValidationError{
			Field:   "content/embeds",
			Message: "at least one of content or embeds is required",
		}
	}

	if len(w.Content) > 2000 {
		return &ValidationError{
			Field:   "content",
			Message: "content exceeds 2000 characters",
		}
	}

	if len(w.Embeds) > 10 {
		return &ValidationError{
			Field:   "embeds",
			Message: "maximum 10 embeds allowed",
		}
	}

	if len(w.ThreadName) > 100 {
		return &ValidationError{
			Field:   "thread_name",
			Message: "thread name exceeds 100 characters",
		}
	}

	if w.ThreadID != "" && w.ThreadName != "" {
		return &ValidationError{
			Field:   "thread_id/thread_name",
			Message: "cannot set both thread_id and thread_name",
		}
	}

	for i, embed := range w.Embeds {
		if err := validateEmbed(&embed); err != nil {
			return err
		}
		_ = i // silence unused variable warning for now
	}

	return nil
}

func validateEmbed(e *Embed) error {
	if len(e.Title) > 256 {
		return &ValidationError{Field: "embed.title", Message: "title exceeds 256 characters"}
	}
	if len(e.Description) > 4096 {
		return &ValidationError{Field: "embed.description", Message: "description exceeds 4096 characters"}
	}
	if len(e.Fields) > 25 {
		return &ValidationError{Field: "embed.fields", Message: "maximum 25 fields allowed"}
	}
	return nil
}
