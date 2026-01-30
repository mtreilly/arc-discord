package gateway

import "testing"

func TestAllIntentsIncludesEverything(t *testing.T) {
	mask := AllIntents()
	if !mask.Has(IntentMessageContent) {
		t.Fatalf("expected all intents to include message content")
	}
	if !mask.Has(IntentGuilds) {
		t.Fatalf("expected all intents to include guilds")
	}
}

func TestDefaultIntentsExcludesPrivileged(t *testing.T) {
	mask := DefaultIntents()
	if mask.Has(IntentMessageContent) {
		t.Fatalf("default intents should not include message content")
	}
	if !mask.Has(IntentGuildMessages) {
		t.Fatalf("default intents should include guild message events")
	}
}

func TestHasHandlesZero(t *testing.T) {
	if !AllIntents().Has(0) {
		t.Fatalf("mask should always report true for zero intent")
	}
}
