package interactions

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestServerHandlesPing(t *testing.T) {
	server, priv := newTestServer(t)

	body, _ := json.Marshal(&types.Interaction{Type: types.InteractionTypePing})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Type != types.InteractionResponsePong {
		t.Fatalf("expected pong response, got %d", resp.Type)
	}
}

func TestServerCommandHandler(t *testing.T) {
	server, priv := newTestServer(t)

	server.RegisterCommand("hello", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return &types.InteractionResponse{
			Type: types.InteractionResponseChannelMessageWithSource,
			Data: &types.InteractionApplicationCommandCallbackData{
				Content: "world",
			},
		}, nil
	})

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "hello"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil || resp.Data.Content != "world" {
		t.Fatalf("unexpected response payload %+v", resp.Data)
	}
}

func TestServerUnknownHandler(t *testing.T) {
	server, priv := newTestServer(t)

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "missing"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServerInvalidSignature(t *testing.T) {
	server, _ := newTestServer(t)

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "hello"},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set(timestampHeader, "123456")
	req.Header.Set(signatureHeader, "deadbeef")

	rr := httptest.NewRecorder()
	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestServerHandlerError(t *testing.T) {
	server, priv := newTestServer(t)
	server.RegisterCommand("fail", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return nil, context.Canceled
	})

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "fail"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestServerComponentHandler(t *testing.T) {
	server, priv := newTestServer(t)
	server.RegisterComponent("confirm", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return NewMessageResponse("confirmed").SetEphemeral(true).Build()
	})

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeMessageComponent,
		Data: &types.InteractionData{CustomID: "confirm"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil || resp.Data.Content != "confirmed" {
		t.Fatalf("unexpected response payload %+v", resp.Data)
	}
	if resp.Data.Flags&interactionResponseFlagEphemeral == 0 {
		t.Fatalf("expected ephemeral flag to be set")
	}
}

func TestServerModalHandler(t *testing.T) {
	server, priv := newTestServer(t)
	server.RegisterModal("modal_submit", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return NewMessageResponse("modal result").Build()
	})

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeModalSubmit,
		Data: &types.InteractionData{CustomID: "modal_submit"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil || resp.Data.Content != "modal result" {
		t.Fatalf("unexpected response payload %+v", resp.Data)
	}
	if resp.Type != types.InteractionResponseChannelMessageWithSource {
		t.Fatalf("expected message response, got %d", resp.Type)
	}
}

func TestServerAutocompleteHandler(t *testing.T) {
	server, priv := newTestServer(t)
	server.RegisterAutocomplete("lookup", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		return &types.InteractionResponse{
			Type: types.InteractionResponseAutocompleteResult,
			Data: &types.InteractionApplicationCommandCallbackData{
				Choices: []types.AutocompleteChoice{
					{Name: "sessions", Value: "sessions"},
				},
			},
		}, nil
	})

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommandAutocomplete,
		Data: &types.InteractionData{Name: "lookup"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp types.InteractionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Type != types.InteractionResponseAutocompleteResult {
		t.Fatalf("expected autocomplete response, got %d", resp.Type)
	}
	if resp.Data == nil || len(resp.Data.Choices) != 1 {
		t.Fatalf("expected autocomplete choices, got %+v", resp.Data)
	}
}

func TestServerWithRouterMiddleware(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	router := NewRouter()
	order := ""
	router.Use(func(next Handler) Handler {
		return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
			order += "A"
			return next(ctx, i)
		}
	})
	router.Use(func(next Handler) Handler {
		return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
			order += "B"
			return next(ctx, i)
		}
	})
	router.Command("ping", func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		order += "C"
		return NewMessageResponse("pong").SetEphemeral(true).Build()
	})

	server, err := NewServer(hex.EncodeToString(pub), WithRouter(router))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	body, _ := json.Marshal(&types.Interaction{
		Type: types.InteractionTypeApplicationCommand,
		Data: &types.InteractionData{Name: "ping"},
	})
	req := newSignedRequest(t, priv, body)
	rr := httptest.NewRecorder()

	server.HandleInteraction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if order != "ABC" {
		t.Fatalf("expected middleware order ABC, got %s", order)
	}
}

func newTestServer(t *testing.T) (*Server, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	s, err := NewServer(hex.EncodeToString(pub))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return s, priv
}

func newSignedRequest(t *testing.T, priv ed25519.PrivateKey, body []byte) *http.Request {
	t.Helper()
	timestamp := "1234567890"
	message := append([]byte(timestamp), body...)
	signature := ed25519.Sign(priv, message)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set(timestampHeader, timestamp)
	req.Header.Set(signatureHeader, hex.EncodeToString(signature))
	return req
}
