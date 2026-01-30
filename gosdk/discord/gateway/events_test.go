package gateway

import (
	"encoding/json"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestReadyEventType(t *testing.T) {
	event := &ReadyEvent{V: 1, User: &types.User{ID: "1", Username: "bot"}}
	if event.Type() != EventReady {
		t.Fatalf("expected %s, got %s", EventReady, event.Type())
	}
}

func TestMessageEventsType(t *testing.T) {
	create := &MessageCreateEvent{Message: &types.Message{ID: "m1"}}
	if create.Type() != EventMessageCreate {
		t.Fatalf("expected %s", EventMessageCreate)
	}
	update := &MessageUpdateEvent{Message: &types.Message{ID: "m1"}}
	if update.Type() != EventMessageUpdate {
		t.Fatalf("expected %s", EventMessageUpdate)
	}
}

func TestGuildDeleteEventSerialization(t *testing.T) {
	event := &GuildDeleteEvent{GuildID: "g1", Unavailable: true}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded["id"] != event.GuildID {
		t.Fatalf("unexpected id: %v", decoded["id"])
	}
	if decoded["unavailable"] != true {
		t.Fatalf("expected unavailable true")
	}
}
