package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestDispatcherDispatchCallsHandlers(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false

	dispatcher.On(EventReady, func(ctx context.Context, event Event) error {
		called = true
		return nil
	})

	if err := dispatcher.Dispatch(context.Background(), &ReadyEvent{V: 1}); err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Fatalf("handler not called")
	}
}

func TestDispatcherTypeSpecificHandler(t *testing.T) {
	dispatcher := NewDispatcher()
	called := false

	dispatcher.OnMessageCreate(func(ctx context.Context, event *MessageCreateEvent) error {
		if event.ID != "msg" {
			return errors.New("unexpected message")
		}
		called = true
		return nil
	})

	if err := dispatcher.Dispatch(context.Background(), &MessageCreateEvent{Message: &types.Message{ID: "msg"}}); err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if !called {
		t.Fatalf("typed handler not invoked")
	}
}

func TestDispatcherCollectsErrors(t *testing.T) {
	dispatcher := NewDispatcher()

	dispatcher.On(EventReady, func(ctx context.Context, event Event) error {
		return errors.New("fail1")
	})
	dispatcher.On(EventReady, func(ctx context.Context, event Event) error {
		return errors.New("fail2")
	})

	err := dispatcher.Dispatch(context.Background(), &ReadyEvent{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
