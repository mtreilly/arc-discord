package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/yourorg/arc-discord/gosdk/logger"
)

// EventHandler processes a gateway event.
type EventHandler func(ctx context.Context, event Event) error

// Dispatcher routes gateway events to registered handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
	logger   *logger.Logger
}

// DispatcherOption configures the dispatcher.
type DispatcherOption func(*Dispatcher)

// WithDispatcherLogger overrides the logger used by the dispatcher.
func WithDispatcherLogger(l *logger.Logger) DispatcherOption {
	return func(d *Dispatcher) {
		if l != nil {
			d.logger = l
		}
	}
}

// NewDispatcher constructs a dispatcher with optional configuration.
func NewDispatcher(opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[string][]EventHandler),
		logger:   logger.Default(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// On registers a handler for the given event type.
func (d *Dispatcher) On(eventType string, handler EventHandler) {
	if eventType == "" || handler == nil {
		return
	}
	d.mu.Lock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
	d.mu.Unlock()
}

// OnMessageCreate registers a handler for MESSAGE_CREATE events.
func (d *Dispatcher) OnMessageCreate(handler func(context.Context, *MessageCreateEvent) error) {
	d.On(EventMessageCreate, func(ctx context.Context, event Event) error {
		evt, ok := event.(*MessageCreateEvent)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return handler(ctx, evt)
	})
}

// OnMessageUpdate registers a handler for MESSAGE_UPDATE events.
func (d *Dispatcher) OnMessageUpdate(handler func(context.Context, *MessageUpdateEvent) error) {
	d.On(EventMessageUpdate, func(ctx context.Context, event Event) error {
		evt, ok := event.(*MessageUpdateEvent)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return handler(ctx, evt)
	})
}

// OnInteraction registers a handler for INTERACTION_CREATE events.
func (d *Dispatcher) OnInteraction(handler func(context.Context, *InteractionCreateEvent) error) {
	d.On(EventInteractionCreate, func(ctx context.Context, event Event) error {
		evt, ok := event.(*InteractionCreateEvent)
		if !ok {
			return fmt.Errorf("unexpected event type %T", event)
		}
		return handler(ctx, evt)
	})
}

// Dispatch invokes handlers for the supplied event.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	if event == nil {
		return nil
	}

	d.mu.RLock()
	handlers := append([]EventHandler(nil), d.handlers[event.Type()]...)
	d.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	var errs []error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			d.logger.Error("event handler error", "event", event.Type(), "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
