package interactions

import (
	"context"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestRouterCommandResolution(t *testing.T) {
	router := NewRouter()
	router.Command("Hello", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return nil, nil
	})

	interaction := &types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "hello"},
	}
	if handler := router.Resolve(interaction); handler == nil {
		t.Fatalf("expected handler to resolve")
	}
}

func TestRouterComponentPattern(t *testing.T) {
	router := NewRouter()
	router.ComponentPattern(`^btn_(\d+)$`, func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return nil, nil
	})

	interaction := &types.Interaction{
		Type: types.InteractionTypeMessageComponent,
		Data: &types.InteractionData{CustomID: "btn_42"},
	}
	if handler := router.Resolve(interaction); handler == nil {
		t.Fatalf("expected pattern handler to resolve")
	}
}

func TestRouterMiddleware(t *testing.T) {
	router := NewRouter()

	callChain := ""
	router.Use(func(next Handler) Handler {
		return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
			callChain += "A"
			return next(ctx, i)
		}
	})
	router.Use(func(next Handler) Handler {
		return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
			callChain += "B"
			return next(ctx, i)
		}
	})

	router.Command("test", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		callChain += "C"
		return nil, nil
	})

	interaction := &types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "test"},
	}

	if handler := router.Resolve(interaction); handler == nil {
		t.Fatalf("expected handler")
	} else {
		handler(context.Background(), interaction)
	}

	if callChain != "ABC" {
		t.Fatalf("expected middleware order ABC, got %s", callChain)
	}
}
