package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yourorg/arc-discord/gosdk/discord/interactions"
	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

const (
	handlerKindCommand      = "command"
	handlerKindComponent    = "component"
	handlerKindModal        = "modal"
	handlerKindAutocomplete = "autocomplete"
	redisPublishTimeout     = 5 * time.Second
)

type handlerBinding struct {
	Kind                string
	Key                 string
	Route               handlerRoute
	AutocompleteChoices []types.AutocompleteChoice
}

type interactionPublisher interface {
	Publish(ctx context.Context, env *redisEnvelope) error
	Close() error
}

func collectHandlerBindings(cfg interactionsConfig) []handlerBinding {
	if !cfg.Enabled {
		return nil
	}
	total := len(cfg.Handlers.Commands) + len(cfg.Handlers.Components) + len(cfg.Handlers.Modals) + len(cfg.Handlers.Autocomplete)
	bindings := make([]handlerBinding, 0, total)
	for key, route := range cfg.Handlers.Commands {
		if route.Agent == "" {
			continue
		}
		bindings = append(bindings, handlerBinding{
			Kind:  handlerKindCommand,
			Key:   strings.ToLower(key),
			Route: route,
		})
	}
	for key, route := range cfg.Handlers.Components {
		if route.Agent == "" {
			continue
		}
		bindings = append(bindings, handlerBinding{
			Kind:  handlerKindComponent,
			Key:   key,
			Route: route,
		})
	}
	for key, route := range cfg.Handlers.Modals {
		if route.Agent == "" {
			continue
		}
		bindings = append(bindings, handlerBinding{
			Kind:  handlerKindModal,
			Key:   key,
			Route: route,
		})
	}
	for key, route := range cfg.Handlers.Autocomplete {
		choices := buildAutocompleteChoices(route.Choices)
		if len(choices) == 0 {
			continue
		}
		bindings = append(bindings, handlerBinding{
			Kind:                handlerKindAutocomplete,
			Key:                 strings.ToLower(key),
			Route:               route,
			AutocompleteChoices: choices,
		})
	}
	return bindings
}

func registerInteractionHandlers(srv *interactions.Server, timeout time.Duration, publisher interactionPublisher, bindings []handlerBinding) error {
	if srv == nil {
		return errors.New("interaction server is not initialized")
	}
	if len(bindings) == 0 {
		return errors.New("no interaction handlers configured (set interactions.handlers in discord.yaml)")
	}
	for _, binding := range bindings {
		handler := dispatchHandler(binding, timeout, publisher)
		switch binding.Kind {
		case handlerKindCommand:
			srv.RegisterCommand(binding.Key, handler)
		case handlerKindComponent:
			srv.RegisterComponent(binding.Key, handler)
		case handlerKindModal:
			srv.RegisterModal(binding.Key, handler)
		case handlerKindAutocomplete:
			srv.RegisterAutocomplete(binding.Key, handler)
		default:
			return fmt.Errorf("unknown handler kind %q", binding.Kind)
		}
	}
	return nil
}

func dispatchHandler(binding handlerBinding, timeout time.Duration, publisher interactionPublisher) interactions.Handler {
	if binding.Kind == handlerKindAutocomplete {
		return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
			if len(binding.AutocompleteChoices) == 0 {
				return nil, fmt.Errorf("autocomplete handler %s missing choices", binding.Key)
			}
			return buildAutocompleteResponse(binding.AutocompleteChoices)
		}
	}
	return func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error) {
		if binding.Route.Agent == "" {
			return nil, fmt.Errorf("interaction handler %s missing agent routing", binding.Key)
		}
		payload, err := newRedisEnvelope(binding, timeout, i)
		if err != nil {
			return nil, err
		}
		if err := publisher.Publish(ctx, payload); err != nil {
			return nil, err
		}
		return buildDeferredResponse()
	}
}

func newRedisEnvelope(binding handlerBinding, timeout time.Duration, interaction *types.Interaction) (*redisEnvelope, error) {
	if interaction == nil {
		return nil, errors.New("interaction payload is nil")
	}
	raw, err := json.Marshal(interaction)
	if err != nil {
		return nil, fmt.Errorf("encode interaction: %w", err)
	}
	env := &redisEnvelope{
		Agent:          binding.Route.Agent,
		Kind:           binding.Kind,
		Key:            binding.Key,
		Interaction:    raw,
		ReceivedAt:     time.Now().UTC(),
		TimeoutSeconds: int(timeout.Seconds()),
		Source:         "vibe.discord.server",
	}
	return env, nil
}

func buildDeferredResponse() (*types.InteractionResponse, error) {
	resp, err := interactions.NewDeferredResponse().Build()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func buildAutocompleteResponse(choices []types.AutocompleteChoice) (*types.InteractionResponse, error) {
	resp := &types.InteractionResponse{
		Type: types.InteractionResponseAutocompleteResult,
		Data: &types.InteractionApplicationCommandCallbackData{
			Choices: choices,
		},
	}
	if err := resp.Validate(); err != nil {
		return nil, err
	}
	return resp, nil
}

func buildAutocompleteChoices(raw []autocompleteChoice) []types.AutocompleteChoice {
	if len(raw) == 0 {
		return nil
	}
	choices := make([]types.AutocompleteChoice, 0, len(raw))
	for _, entry := range raw {
		if strings.TrimSpace(entry.Name) == "" || entry.Value == nil {
			continue
		}
		choices = append(choices, types.AutocompleteChoice{
			Name:  entry.Name,
			Value: entry.Value,
		})
	}
	return choices
}

type redisPublisher struct {
	client *redis.Client
	prefix string
}

func newRedisPublisher(cfg redisConfig) (*redisPublisher, error) {
	client := redis.NewClient(newRedisOptions(cfg))
	ctx, cancel := context.WithTimeout(context.Background(), redisPublishTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	prefix := normalizeChannelPrefix(cfg.ChannelPrefix)
	return &redisPublisher{client: client, prefix: prefix}, nil
}

func (p *redisPublisher) Publish(ctx context.Context, env *redisEnvelope) error {
	if env == nil {
		return errors.New("missing envelope")
	}
	if strings.TrimSpace(env.Agent) == "" {
		return errors.New("envelope missing agent")
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	channel := fmt.Sprintf("%s:agent:%s", p.prefix, strings.ToLower(env.Agent))
	pubCtx, cancel := context.WithTimeout(ctx, redisPublishTimeout)
	defer cancel()
	if err := p.client.Publish(pubCtx, channel, payload).Err(); err != nil {
		return fmt.Errorf("publish redis channel %s: %w", channel, err)
	}
	return nil
}

func (p *redisPublisher) Close() error {
	return p.client.Close()
}
