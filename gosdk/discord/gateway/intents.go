package gateway

// Intent enumerates Discord gateway intents.
type Intent int

const (
	IntentGuilds Intent = 1 << iota
	IntentGuildMembers
	IntentGuildBans
	IntentGuildEmojis
	IntentGuildIntegrations
	IntentGuildWebhooks
	IntentGuildInvites
	IntentGuildVoiceStates
	IntentGuildPresences
	IntentGuildMessages
	IntentGuildMessageReactions
	IntentGuildMessageTyping
	IntentDirectMessages
	IntentDirectMessageReactions
	IntentDirectMessageTyping
	IntentMessageContent
	IntentGuildScheduledEvents
	IntentAutoModerationConfiguration
	IntentAutoModerationExecution
)

// AllIntents returns a mask with every intent enabled.
func AllIntents() Intent {
	return IntentGuilds | IntentGuildMembers | IntentGuildBans | IntentGuildEmojis |
		IntentGuildIntegrations | IntentGuildWebhooks | IntentGuildInvites | IntentGuildVoiceStates |
		IntentGuildPresences | IntentGuildMessages | IntentGuildMessageReactions | IntentGuildMessageTyping |
		IntentDirectMessages | IntentDirectMessageReactions | IntentDirectMessageTyping | IntentMessageContent |
		IntentGuildScheduledEvents | IntentAutoModerationConfiguration | IntentAutoModerationExecution
}

// DefaultIntents returns a safe intent mask for non-privileged bots.
func DefaultIntents() Intent {
	return IntentGuilds | IntentGuildMembers | IntentGuildMessages | IntentGuildMessageReactions |
		IntentGuildMessageTyping | IntentDirectMessages | IntentDirectMessageReactions | IntentDirectMessageTyping
}

// Has returns true if the mask includes the requested intent.
func (i Intent) Has(intent Intent) bool {
	if intent == 0 {
		return true
	}
	return i&intent == intent
}
