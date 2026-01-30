package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
	"github.com/yourorg/arc-discord/gosdk/logger"
)

// Activity represents a Discord presence activity.
type Activity struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

// PresenceUpdate describes the payload sent to the gateway.
type PresenceUpdate struct {
	Since      *int       `json:"since,omitempty"`
	Activities []Activity `json:"activities,omitempty"`
	Status     string     `json:"status,omitempty"`
	AFK        bool       `json:"afk"`
}

// ClientOption configures the gateway client.
type ClientOption func(*Client)

// WithDispatcher swaps the event dispatcher.
func WithDispatcher(d *Dispatcher) ClientOption {
	return func(c *Client) {
		if d != nil {
			c.dispatcher = d
		}
	}
}

// WithGatewayLogger overrides the logger.
func WithGatewayLogger(l *logger.Logger) ClientOption {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithConnection allows providing a pre-configured connection.
func WithConnection(conn *Connection) ClientOption {
	return func(c *Client) {
		if conn != nil {
			c.conn = conn
		}
	}
}

// WithConnectionOptions augments the underlying connection options.
func WithConnectionOptions(opts ...ConnectionOption) ClientOption {
	return func(c *Client) {
		c.connectionOpts = append(c.connectionOpts, opts...)
	}
}

// Client manages a gateway connection and event routing.
type Client struct {
	token          string
	intents        int
	conn           *Connection
	dispatcher     *Dispatcher
	logger         *logger.Logger
	status         string
	activity       *Activity
	connectionOpts []ConnectionOption

	eventCancel context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// NewClient builds a gateway client configured with the given token and intents.
func NewClient(token string, intents int, opts ...ClientOption) (*Client, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	c := &Client{
		token:      token,
		intents:    intents,
		dispatcher: NewDispatcher(),
		logger:     logger.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.conn == nil {
		conn, err := NewConnection(token, intents, c.connectionOpts...)
		if err != nil {
			return nil, err
		}
		c.conn = conn
	}

	return c, nil
}

// Connect opens the gateway connection and starts event processing.
func (c *Client) Connect(ctx context.Context) error {
	if c.conn == nil {
		return errors.New("connection is not configured")
	}
	if c.eventCancel != nil {
		return errors.New("client already connected")
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.eventCancel = cancel

	if err := c.conn.Connect(runCtx); err != nil {
		cancel()
		return err
	}

	c.wg.Add(1)
	go c.run(runCtx)

	if err := c.identify(runCtx); err != nil {
		c.logger.Warn("identify failed", "error", err)
		cancel()
		c.wg.Wait()
		return err
	}

	if c.status != "" || c.activity != nil {
		if err := c.UpdatePresence(runCtx, c.status, c.activity); err != nil {
			c.logger.Warn("restore presence failed", "error", err)
		}
	}

	return nil
}

// Disconnect closes the gateway connection and waits for the read loop.
func (c *Client) Disconnect() error {
	if c.eventCancel != nil {
		c.eventCancel()
		c.eventCancel = nil
	}
	c.wg.Wait()
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// On registers a generic event handler.
func (c *Client) On(eventType string, handler EventHandler) {
	c.dispatcher.On(eventType, handler)
}

// OnMessageCreate registers a MESSAGE_CREATE handler.
func (c *Client) OnMessageCreate(handler func(context.Context, *MessageCreateEvent) error) {
	c.dispatcher.OnMessageCreate(handler)
}

// OnMessageUpdate registers a MESSAGE_UPDATE handler.
func (c *Client) OnMessageUpdate(handler func(context.Context, *MessageUpdateEvent) error) {
	c.dispatcher.OnMessageUpdate(handler)
}

// OnInteraction registers an INTERACTION_CREATE handler.
func (c *Client) OnInteraction(handler func(context.Context, *InteractionCreateEvent) error) {
	c.dispatcher.OnInteraction(handler)
}

// UpdatePresence sends a presence update to the gateway and remembers the desired state.
func (c *Client) UpdatePresence(ctx context.Context, status string, activity *Activity) error {
	c.mu.Lock()
	c.status = status
	c.activity = activity
	c.mu.Unlock()

	if c.conn == nil {
		return errors.New("client is not connected")
	}

	update := PresenceUpdate{
		Status: status,
		AFK:    false,
	}
	if activity != nil {
		update.Activities = []Activity{*activity}
	}

	payload := &Payload{Op: OpCodePresenceUpdate}
	raw, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal presence update: %w", err)
	}
	payload.D = raw

	return c.conn.Send(ctx, payload)
}

// RequestGuildMembers sends a GUILD_MEMBERS request to the gateway.
func (c *Client) RequestGuildMembers(ctx context.Context, guildID string, query string, limit int) error {
	if guildID == "" {
		return errors.New("guild_id is required")
	}

	payload := &Payload{Op: OpCodeRequestGuildMembers}
	data := map[string]interface{}{
		"guild_id": guildID,
	}
	if query != "" {
		data["query"] = query
	}
	if limit > 0 {
		data["limit"] = limit
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal guild member request: %w", err)
	}
	payload.D = raw
	return c.conn.Send(ctx, payload)
}

// Send proxies a raw payload over the websocket connection.
func (c *Client) Send(ctx context.Context, payload *Payload) error {
	if c.conn == nil {
		return errors.New("client not connected")
	}
	return c.conn.Send(ctx, payload)
}

func (c *Client) identify(ctx context.Context) error {
	payload := &Payload{Op: OpCodeIdentify}
	props := IdentifyPayload{
		Token: c.token,
		Properties: IdentifyProperties{
			OS:      runtime.GOOS,
			Browser: "agent-discord",
			Device:  "agent-discord",
		},
		Intents: c.intents,
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("marshal identify: %w", err)
	}
	payload.D = raw
	return c.conn.Send(ctx, payload)
}

func (c *Client) run(ctx context.Context) {
	defer c.wg.Done()

	for {
		payload, err := c.conn.Receive(ctx)
		if err != nil {
			if ctx.Err() == nil {
				c.logger.Warn("gateway receive failed", "error", err)
			}
			return
		}

		switch payload.Op {
		case OpCodeDispatch:
			c.handleDispatch(ctx, payload)
		case OpCodeHello:
			c.handleHello(ctx, payload)
		case OpCodeReconnect:
			go c.handleReconnect(ctx)
		case OpCodeInvalidSession:
			c.conn.SetSession("")
			if err := c.identify(ctx); err != nil {
				c.logger.Warn("identify after invalid session failed", "error", err)
			}
		}
	}
}

func (c *Client) handleDispatch(ctx context.Context, payload *Payload) {
	event, err := decodeEvent(payload)
	if err != nil {
		c.logger.Warn("decode event failed", "error", err)
		return
	}
	if event == nil {
		return
	}

	if ready, ok := event.(*ReadyEvent); ok && ready.SessionID != "" {
		c.conn.SetSession(ready.SessionID)
	}

	if err := c.dispatcher.Dispatch(ctx, event); err != nil {
		c.logger.Warn("dispatch error", "error", err)
	}
}

func (c *Client) handleHello(ctx context.Context, payload *Payload) {
	var hello struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := json.Unmarshal(payload.D, &hello); err != nil {
		c.logger.Warn("failed to parse hello", "error", err)
		return
	}
	if hello.HeartbeatInterval > 0 {
		c.conn.heartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond
		c.conn.stopHeartbeat()
		c.conn.startHeartbeat(ctx)
	}
}

func (c *Client) handleReconnect(ctx context.Context) {
	c.logger.Info("gateway requested reconnect")
	if err := c.conn.reconnect(ctx); err != nil {
		c.logger.Warn("reconnect failed", "error", err)
		return
	}
	if err := c.identify(ctx); err != nil {
		c.logger.Warn("identify after reconnect failed", "error", err)
		return
	}
	if err := c.UpdatePresence(ctx, c.status, c.activity); err != nil {
		c.logger.Warn("restore presence failed", "error", err)
	}
}

func decodeEvent(payload *Payload) (Event, error) {
	if payload == nil || payload.Op != OpCodeDispatch || payload.T == "" {
		return nil, nil
	}

	switch payload.T {
	case EventReady:
		var evt ReadyEvent
		if err := json.Unmarshal(payload.D, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case EventMessageCreate:
		var msg types.Message
		if err := json.Unmarshal(payload.D, &msg); err != nil {
			return nil, err
		}
		return &MessageCreateEvent{Message: &msg}, nil
	case EventMessageUpdate:
		var msg types.Message
		if err := json.Unmarshal(payload.D, &msg); err != nil {
			return nil, err
		}
		return &MessageUpdateEvent{Message: &msg}, nil
	case EventMessageDelete:
		var evt MessageDeleteEvent
		if err := json.Unmarshal(payload.D, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case EventGuildCreate:
		var guild types.Guild
		if err := json.Unmarshal(payload.D, &guild); err != nil {
			return nil, err
		}
		return &GuildCreateEvent{Guild: &guild}, nil
	case EventGuildUpdate:
		var guild types.Guild
		if err := json.Unmarshal(payload.D, &guild); err != nil {
			return nil, err
		}
		return &GuildUpdateEvent{Guild: &guild}, nil
	case EventGuildDelete:
		var evt GuildDeleteEvent
		if err := json.Unmarshal(payload.D, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case EventInteractionCreate:
		var interaction types.Interaction
		if err := json.Unmarshal(payload.D, &interaction); err != nil {
			return nil, err
		}
		return &InteractionCreateEvent{Interaction: &interaction}, nil
	default:
		return nil, nil
	}
}
