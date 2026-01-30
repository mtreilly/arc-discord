package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourorg/arc-discord/gosdk/logger"
)

const (
	defaultGatewayURL        = "wss://gateway.discord.gg/?v=10&encoding=json"
	defaultHeartbeatInterval = 41_250 * time.Millisecond
)

type ConnectionOption func(*Connection)

type Connection struct {
	token             string
	intents           int
	gatewayURL        string
	dialer            *websocket.Dialer
	logger            *logger.Logger
	writeMu           sync.Mutex
	conn              *websocket.Conn
	mu                sync.Mutex
	sequence          int
	sessionID         string
	heartbeatTicker   *time.Ticker
	heartbeatCtx      context.Context
	heartbeatCancel   context.CancelFunc
	heartbeatInterval time.Duration
}

func WithGatewayURL(url string) ConnectionOption {
	return func(c *Connection) {
		if url != "" {
			c.gatewayURL = url
		}
	}
}

func WithLogger(l *logger.Logger) ConnectionOption {
	return func(c *Connection) {
		if l != nil {
			c.logger = l
		}
	}
}

func WithDialer(d *websocket.Dialer) ConnectionOption {
	return func(c *Connection) {
		if d != nil {
			c.dialer = d
		}
	}
}

func WithHeartbeatInterval(interval time.Duration) ConnectionOption {
	return func(c *Connection) {
		if interval > 0 {
			c.heartbeatInterval = interval
		}
	}
}

func NewConnection(token string, intents int, opts ...ConnectionOption) (*Connection, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	c := &Connection{
		token:             token,
		intents:           intents,
		gatewayURL:        defaultGatewayURL,
		dialer:            websocket.DefaultDialer,
		logger:            logger.Default(),
		heartbeatInterval: defaultHeartbeatInterval,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.gatewayURL == "" {
		c.gatewayURL = defaultGatewayURL
	}
	return c, nil
}

func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.conn != nil {
		c.mu.Unlock()
		return errors.New("already connected")
	}
	c.mu.Unlock()

	headers := http.Header{}
	headers.Set("User-Agent", "agent-discord-gateway/1.0")

	conn, _, err := c.dialer.DialContext(ctx, c.gatewayURL, headers)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.logger.Info("gateway connected", "url", c.gatewayURL)
	c.startHeartbeat(ctx)
	return nil
}

func (c *Connection) Close() error {
	c.stopHeartbeat()

	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (c *Connection) Send(ctx context.Context, payload *Payload) error {
	if payload == nil {
		return errors.New("payload is required")
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return errors.New("not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func (c *Connection) Receive(ctx context.Context) (*Payload, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, errors.New("not connected")
	}

	var payload Payload
	if err := conn.ReadJSON(&payload); err != nil {
		return nil, err
	}

	if payload.S > 0 {
		c.mu.Lock()
		c.sequence = payload.S
		c.mu.Unlock()
	}

	return &payload, nil
}

func (c *Connection) startHeartbeat(ctx context.Context) {
	c.mu.Lock()
	if c.heartbeatCtx != nil {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	c.heartbeatCtx = ctx
	c.heartbeatCancel = cancel
	c.heartbeatTicker = time.NewTicker(c.heartbeatInterval)
	c.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.heartbeatTicker.C:
				if err := c.sendHeartbeat(ctx); err != nil {
					c.logger.Warn("heartbeat failed", "error", err)
				}
			}
		}
	}()
}

func (c *Connection) stopHeartbeat() {
	c.mu.Lock()
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
		c.heartbeatTicker = nil
	}
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}
	c.heartbeatCtx = nil
	c.heartbeatCancel = nil
	c.mu.Unlock()
}

func (c *Connection) sendHeartbeat(ctx context.Context) error {
	c.mu.Lock()
	seq := c.sequence
	c.mu.Unlock()

	var data json.RawMessage
	if seq > 0 {
		data = json.RawMessage(fmt.Sprintf("%d", seq))
	} else {
		data = json.RawMessage("null")
	}
	payload := &Payload{Op: OpCodeHeartbeat, D: data}
	return c.Send(ctx, payload)
}

func (c *Connection) reconnect(ctx context.Context) error {
	if err := c.Close(); err != nil {
		c.logger.Warn("failed to close before reconnect", "error", err)
	}
	if err := c.Connect(ctx); err != nil {
		return err
	}

	if c.sessionID != "" {
		return c.resume(ctx)
	}
	return nil
}

func (c *Connection) resume(ctx context.Context) error {
	c.mu.Lock()
	session := c.sessionID
	seq := c.sequence
	c.mu.Unlock()

	if session == "" {
		return errors.New("session id required to resume")
	}

	payload := &Payload{Op: OpCodeResume}
	state := map[string]interface{}{
		"token":      c.token,
		"session_id": session,
		"seq":        seq,
	}
	raw, _ := json.Marshal(state)
	payload.D = raw
	return c.Send(ctx, payload)
}

func (c *Connection) SetSession(sessionID string) {
	c.mu.Lock()
	c.sessionID = sessionID
	c.mu.Unlock()
}

func (c *Connection) SetSequence(seq int) {
	c.mu.Lock()
	c.sequence = seq
	c.mu.Unlock()
}
