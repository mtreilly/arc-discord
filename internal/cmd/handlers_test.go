package cmd

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestCollectHandlerBindings(t *testing.T) {
	cfg := interactionsConfig{
		Enabled: true,
		Handlers: handlerMappings{
			Commands: map[string]handlerRoute{
				"help": {Agent: "claude"},
			},
			Components: map[string]handlerRoute{
				"confirm:yes": {Agent: "codex"},
			},
			Modals: map[string]handlerRoute{
				"feedback": {Agent: "codex"},
			},
			Autocomplete: map[string]handlerRoute{
				"session_find": {
					Choices: []autocompleteChoice{
						{Name: "recent", Value: "recent"},
					},
				},
			},
		},
	}
	bindings := collectHandlerBindings(cfg)
	if len(bindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(bindings))
	}
	var sawAutocomplete bool
	for _, b := range bindings {
		if b.Route.Agent == "" {
			if b.Kind != handlerKindAutocomplete {
				t.Fatalf("binding missing agent: %#v", b)
			}
		}
		switch b.Kind {
		case handlerKindCommand, handlerKindComponent, handlerKindModal:
		default:
			if b.Kind != handlerKindAutocomplete {
				t.Fatalf("unexpected kind %s", b.Kind)
			}
			sawAutocomplete = len(b.AutocompleteChoices) == 1
		}
	}
	if !sawAutocomplete {
		t.Fatalf("autocomplete binding missing choices")
	}
}

func TestDispatchHandlerAutocomplete(t *testing.T) {
	binding := handlerBinding{
		Kind: handlerKindAutocomplete,
		Key:  "session_find",
		AutocompleteChoices: []types.AutocompleteChoice{
			{Name: "recent", Value: "recent"},
		},
	}
	handler := dispatchHandler(binding, 0, nil)
	resp, err := handler(context.Background(), &types.Interaction{
		Type: types.InteractionTypeApplicationCommandAutocomplete,
	})
	if err != nil {
		t.Fatalf("autocomplete handler error: %v", err)
	}
	if resp.Type != types.InteractionResponseAutocompleteResult {
		t.Fatalf("expected autocomplete response, got %d", resp.Type)
	}
	if resp.Data == nil || len(resp.Data.Choices) != 1 || resp.Data.Choices[0].Name != "recent" {
		t.Fatalf("unexpected autocomplete choices: %+v", resp.Data)
	}
}

func TestRegisterInteractionHandlersRequiresBindings(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv, err := interactions.NewServer(hex.EncodeToString(pub))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	err = registerInteractionHandlers(srv, time.Second, noopPublisher{}, nil)
	if err == nil || !strings.Contains(err.Error(), "no interaction handlers configured") {
		t.Fatalf("expected error for missing handlers, got %v", err)
	}
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, *redisEnvelope) error { return nil }
func (noopPublisher) Close() error                                  { return nil }
