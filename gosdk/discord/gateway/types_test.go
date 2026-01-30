package gateway

import (
	"encoding/json"
	"testing"
)

func TestOpCodeConstants(t *testing.T) {
	if OpCodeResume != 6 {
		t.Fatalf("expected OpCodeResume to be 6, got %d", OpCodeResume)
	}
}

func TestIdentifyPayloadJSON(t *testing.T) {
	payload := IdentifyPayload{
		Token: "token",
		Properties: IdentifyProperties{
			OS:      "linux",
			Browser: "vibe",
			Device:  "agent",
		},
		Compress: true,
		Intents:  512,
		Shard:    []int{0, 2},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded["token"] != payload.Token {
		t.Fatalf("token mismatch: %v", decoded["token"])
	}
	if decoded["compress"] != true {
		t.Fatalf("compress flag missing")
	}
}

func TestPayloadSerialization(t *testing.T) {
	event := map[string]string{"event": "READY"}
	rawEvent, _ := json.Marshal(event)

	payload := Payload{
		Op: OpCodeDispatch,
		D:  rawEvent,
		S:  1,
		T:  "READY",
	}

	serialized, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("payload marshal failed: %v", err)
	}

	var roundtrip Payload
	if err := json.Unmarshal(serialized, &roundtrip); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if roundtrip.Op != payload.Op || roundtrip.T != payload.T || roundtrip.S != payload.S {
		t.Fatalf("payload roundtrip mismatch: %+v", roundtrip)
	}
}
