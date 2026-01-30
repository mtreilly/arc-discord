package interactions

import (
	"regexp"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Middleware wraps handlers for shared concerns (logging, recovery, etc).
type Middleware func(Handler) Handler

// Router routes interactions to handlers by command/component/modal identifiers.
type Router struct {
	commands          map[string]Handler
	components        map[string]Handler
	modals            map[string]Handler
	autocomplete      map[string]Handler
	componentPatterns []patternHandler
	middleware        []Middleware
}

type patternHandler struct {
	pattern *regexp.Regexp
	handler Handler
}

// NewRouter constructs a new router instance.
func NewRouter() *Router {
	return &Router{
		commands:     make(map[string]Handler),
		components:   make(map[string]Handler),
		modals:       make(map[string]Handler),
		autocomplete: make(map[string]Handler),
	}
}

// Use appends middleware to the router chain.
func (r *Router) Use(m Middleware) {
	if m == nil {
		return
	}
	r.middleware = append(r.middleware, m)
}

// Command registers a handler for a slash/user/message command.
func (r *Router) Command(name string, handler Handler) {
	if r == nil || name == "" || handler == nil {
		return
	}
	r.commands[strings.ToLower(name)] = handler
}

// Component registers a handler for an exact component custom ID.
func (r *Router) Component(customID string, handler Handler) {
	if r == nil || customID == "" || handler == nil {
		return
	}
	r.components[customID] = handler
}

// ComponentPattern registers a handler with a regex pattern that matches component custom IDs.
func (r *Router) ComponentPattern(pattern string, handler Handler) {
	if r == nil || pattern == "" || handler == nil {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	r.componentPatterns = append(r.componentPatterns, patternHandler{
		pattern: re,
		handler: handler,
	})
}

// Modal registers a handler for a modal custom ID.
func (r *Router) Modal(customID string, handler Handler) {
	if r == nil || customID == "" || handler == nil {
		return
	}
	r.modals[customID] = handler
}

// Autocomplete registers a handler for an autocomplete-capable command.
func (r *Router) Autocomplete(name string, handler Handler) {
	if r == nil || name == "" || handler == nil {
		return
	}
	r.autocomplete[strings.ToLower(name)] = handler
}

// Resolve returns a handler for the provided interaction, applying middleware if present.
func (r *Router) Resolve(interaction *types.Interaction) Handler {
	if r == nil || interaction == nil || interaction.Data == nil {
		return nil
	}

	var handler Handler
	switch interaction.Type {
	case types.InteractionTypeApplicationCommand:
		if interaction.Data.Name == "" {
			return nil
		}
		handler = r.commands[strings.ToLower(interaction.Data.Name)]
	case types.InteractionTypeMessageComponent:
		if interaction.Data.CustomID == "" {
			return nil
		}
		handler = r.components[interaction.Data.CustomID]
		if handler == nil {
			for _, pattern := range r.componentPatterns {
				if pattern.pattern.MatchString(interaction.Data.CustomID) {
					handler = pattern.handler
					break
				}
			}
		}
	case types.InteractionTypeModalSubmit:
		if interaction.Data.CustomID == "" {
			return nil
		}
		handler = r.modals[interaction.Data.CustomID]
	case types.InteractionTypeApplicationCommandAutocomplete:
		if interaction.Data.Name == "" {
			return nil
		}
		handler = r.autocomplete[strings.ToLower(interaction.Data.Name)]
	default:
		return nil
	}

	if handler == nil {
		return nil
	}

	return r.applyMiddleware(handler)
}

func (r *Router) applyMiddleware(handler Handler) Handler {
	wrapped := handler
	for i := len(r.middleware) - 1; i >= 0; i-- {
		wrapped = r.middleware[i](wrapped)
	}
	return wrapped
}
