package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsURL(s *httptest.Server) string {
	if strings.HasPrefix(s.URL, "https://") {
		return "wss" + s.URL[5:]
	}
	return "ws" + s.URL[4:]
}

func TestConnectionHeartbeatLifecycle(t *testing.T) {
	upgrader := websocket.Upgrader{}
	ackReceived := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		var payload Payload
		if err := conn.ReadJSON(&payload); err != nil {
			t.Fatalf("read json: %v", err)
		}
		if payload.Op != OpCodeHeartbeat {
			t.Fatalf("expected heartbeat, got %d", payload.Op)
		}

		if err := conn.WriteJSON(Payload{Op: OpCodeHeartbeatAck, S: 1}); err != nil {
			t.Fatalf("write ack: %v", err)
		}
		close(ackReceived)
	}))
	defer server.Close()

	conn, err := NewConnection("token", 0,
		WithGatewayURL(wsURL(server)),
		WithHeartbeatInterval(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("new connection error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := conn.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Receive(ctx); err != nil {
		t.Fatalf("receive error: %v", err)
	}

	select {
	case <-ackReceived:
	case <-ctx.Done():
		t.Fatalf("did not observe ack")
	}
}

func TestConnectionResumePayload(t *testing.T) {
	upgrader := websocket.Upgrader{}
	resumeCh := make(chan *Payload, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		var payload Payload
		if err := conn.ReadJSON(&payload); err != nil {
			t.Fatalf("read json: %v", err)
		}
		resumeCh <- &payload
	}))
	defer server.Close()

	conn, err := NewConnection("token", 0,
		WithGatewayURL(wsURL(server)),
		WithHeartbeatInterval(time.Hour), // prevent automatic heartbeats during the test
	)
	if err != nil {
		t.Fatalf("new connection error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := conn.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	conn.SetSession("session-123")
	conn.SetSequence(42)

	if err := conn.resume(ctx); err != nil {
		t.Fatalf("resume error: %v", err)
	}

	select {
	case payload := <-resumeCh:
		if payload.Op != OpCodeResume {
			t.Fatalf("expected resume opcode, got %d", payload.Op)
		}
		var state map[string]interface{}
		if err := json.Unmarshal(payload.D, &state); err != nil {
			t.Fatalf("unmarshal resume payload: %v", err)
		}
		if state["session_id"] != "session-123" {
			t.Fatalf("unexpected session id %v", state["session_id"])
		}
		if state["seq"] != float64(42) {
			t.Fatalf("unexpected seq %v", state["seq"])
		}
	case <-ctx.Done():
		t.Fatalf("did not observe resume payload")
	}
}
