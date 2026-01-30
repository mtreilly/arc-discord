package gateway

import (
	"encoding/json"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestDecodeEventReady(t *testing.T) {
	raw := map[string]interface{}{
		"v":          1,
		"user":       map[string]string{"id": "1", "username": "bot"},
		"session_id": "session",
	}
	data, _ := json.Marshal(raw)

	payload := &Payload{Op: OpCodeDispatch, T: EventReady, D: data}
	event, err := decodeEvent(payload)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := event.(*ReadyEvent); !ok {
		t.Fatalf("expected ReadyEvent, got %T", event)
	}
}

func TestDecodeEventUnknown(t *testing.T) {
	payload := &Payload{Op: OpCodeDispatch, T: "UNKNOWN"}
	event, err := decodeEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Fatalf("expected nil event, got %T", event)
	}
}

func TestDecodeMessageCreate(t *testing.T) {
	message := types.Message{ID: "msg"}
	data, _ := json.Marshal(message)
	payload := &Payload{Op: OpCodeDispatch, T: EventMessageCreate, D: data}
	event, err := decodeEvent(payload)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if msgEvent, ok := event.(*MessageCreateEvent); !ok || msgEvent.Message.ID != "msg" {
		t.Fatalf("unexpected event %T", event)
	}
}
