package interactions

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/logger"
)

const (
	signatureHeader = "X-Signature-Ed25519"
	timestampHeader = "X-Signature-Timestamp"
)

// Handler processes an interaction and returns an optional response payload.
type Handler func(ctx context.Context, i *types.Interaction) (*types.InteractionResponse, error)

// Server handles HTTP interaction callbacks from Discord.
type Server struct {
	publicKey ed25519.PublicKey
	logger    *logger.Logger
	dryRun    bool
	router    *Router

	commandHandlers      map[string]Handler
	componentHandlers    map[string]Handler
	modalHandlers        map[string]Handler
	autocompleteHandlers map[string]Handler
}

// ServerOption configures additional server behaviour.
type ServerOption func(*Server)

// WithLogger overrides the server logger.
func WithLogger(l *logger.Logger) ServerOption {
	return func(s *Server) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithDryRun skips signature verification (useful for local tests).
func WithDryRun(enabled bool) ServerOption {
	return func(s *Server) {
		s.dryRun = enabled
	}
}

// WithRouter injects a custom router implementation.
func WithRouter(r *Router) ServerOption {
	return func(s *Server) {
		if r != nil {
			s.router = r
		}
	}
}

// NewServer constructs a new interaction server.
func NewServer(publicKey string, opts ...ServerOption) (*Server, error) {
	pubBytes, err := hex.DecodeString(strings.TrimSpace(publicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: expected %d bytes", ed25519.PublicKeySize)
	}

	s := &Server{
		publicKey:            ed25519.PublicKey(pubBytes),
		logger:               logger.Default(),
		commandHandlers:      make(map[string]Handler),
		componentHandlers:    make(map[string]Handler),
		modalHandlers:        make(map[string]Handler),
		autocompleteHandlers: make(map[string]Handler),
		router:               NewRouter(),
	}

	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// RegisterCommand registers a handler for an application command (slash/user/message).
func (s *Server) RegisterCommand(name string, handler Handler) {
	if name == "" || handler == nil {
		return
	}
	s.commandHandlers[strings.ToLower(name)] = handler
	if s.router != nil {
		s.router.Command(name, handler)
	}
}

// RegisterComponent registers a handler for a component custom ID.
func (s *Server) RegisterComponent(customID string, handler Handler) {
	if customID == "" || handler == nil {
		return
	}
	s.componentHandlers[customID] = handler
	if s.router != nil {
		s.router.Component(customID, handler)
	}
}

// RegisterModal registers a handler for a modal custom ID.
func (s *Server) RegisterModal(customID string, handler Handler) {
	if customID == "" || handler == nil {
		return
	}
	s.modalHandlers[customID] = handler
	if s.router != nil {
		s.router.Modal(customID, handler)
	}
}

// RegisterAutocomplete registers a handler for command autocomplete events.
func (s *Server) RegisterAutocomplete(name string, handler Handler) {
	if name == "" || handler == nil {
		return
	}
	s.autocompleteHandlers[strings.ToLower(name)] = handler
	if s.router != nil {
		s.router.Autocomplete(name, handler)
	}
}

// HandleInteraction handles HTTP requests from Discord's interaction endpoint.
func (s *Server) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("failed to read request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !s.dryRun {
		if ok := s.verifyRequest(r, body); !ok {
			http.Error(w, "invalid request signature", http.StatusUnauthorized)
			return
		}
	}

	var interaction types.Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		s.logger.Error("failed to decode interaction", "error", err)
		http.Error(w, "invalid interaction payload", http.StatusBadRequest)
		return
	}

	if interaction.Type == types.InteractionTypePing {
		s.writeJSON(w, http.StatusOK, &types.InteractionResponse{Type: types.InteractionResponsePong})
		return
	}

	handler := s.resolveHandler(&interaction)
	if handler == nil {
		http.Error(w, "handler not found", http.StatusNotFound)
		return
	}

	resp, err := handler(r.Context(), &interaction)
	if err != nil {
		s.logger.Error("interaction handler error", "error", err)
		http.Error(w, "handler error", http.StatusInternalServerError)
		return
	}

	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.writeJSON(w, http.StatusOK, resp); err != nil {
		s.logger.Error("failed to write interaction response", "error", err)
	}
}

func (s *Server) verifyRequest(r *http.Request, body []byte) bool {
	signatureHex := r.Header.Get(signatureHeader)
	timestamp := r.Header.Get(timestampHeader)
	if signatureHex == "" || timestamp == "" {
		return false
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}

	message := append([]byte(timestamp), body...)
	return ed25519.Verify(s.publicKey, message, signature)
}

func (s *Server) resolveHandler(i *types.Interaction) Handler {
	if s.router != nil {
		if handler := s.router.Resolve(i); handler != nil {
			return handler
		}
	}
	if i == nil || i.Data == nil {
		return nil
	}
	switch i.Type {
	case types.InteractionTypeApplicationCommand:
		if i.Data.Name == "" {
			return nil
		}
		return s.commandHandlers[strings.ToLower(i.Data.Name)]
	case types.InteractionTypeMessageComponent:
		return s.componentHandlers[i.Data.CustomID]
	case types.InteractionTypeModalSubmit:
		return s.modalHandlers[i.Data.CustomID]
	case types.InteractionTypeApplicationCommandAutocomplete:
		if i.Data.Name == "" {
			return nil
		}
		return s.autocompleteHandlers[strings.ToLower(i.Data.Name)]
	default:
		return nil
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}
