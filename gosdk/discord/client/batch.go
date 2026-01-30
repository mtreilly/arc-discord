package client

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

func (b *Batcher) collect(batch *[]*batchRequest) {
	for {
		select {
		case req := <-b.queue:
			if req == nil {
				return
			}
			*batch = append(*batch, req)
			if len(*batch) >= b.batchSize {
				return
			}
		default:
			return
		}
	}
}

const (
	defaultBatchSize     = 10
	defaultFlushInterval = 250 * time.Millisecond
)

// BatcherOption configures a request batcher.
type BatcherOption func(*Batcher)

// WithBatchSize sets the number of requests per batch.
func WithBatchSize(n int) BatcherOption {
	return func(b *Batcher) {
		if n > 0 {
			b.batchSize = n
		}
	}
}

// WithFlushInterval sets how frequently the batch flushes.
func WithFlushInterval(d time.Duration) BatcherOption {
	return func(b *Batcher) {
		if d > 0 {
			b.flushInterval = d
		}
	}
}

// Batcher groups outgoing requests in configurable batches.
type Batcher struct {
	client        *Client
	batchSize     int
	flushInterval time.Duration
	queue         chan *batchRequest
	flushCh       chan chan error
	stopCh        chan struct{}
	doneCh        chan struct{}
	once          sync.Once
}

type batchRequest struct {
	ctx  context.Context
	exec func(context.Context) error
}

// NewBatcher creates a batcher wired to the client.
func (c *Client) NewBatcher(opts ...BatcherOption) *Batcher {
	b := &Batcher{
		client:        c,
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		queue:         make(chan *batchRequest, 100),
		flushCh:       make(chan chan error),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(b)
	}
	go b.run()
	return b
}

// AddMessage enqueues a create message request.
func (b *Batcher) AddMessage(ctx context.Context, channelID, content string) error {
	body := map[string]string{"content": content}
	path := fmt.Sprintf("channels/%s/messages", channelID)
	return b.enqueue(ctx, path, http.MethodPost, body)
}

// AddReaction enqueues an emoji reaction.
func (b *Batcher) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	path := fmt.Sprintf("channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, emoji)
	return b.enqueue(ctx, path, http.MethodPut, nil)
}

// enqueue pushes a request into the batch.
func (b *Batcher) enqueue(ctx context.Context, path, method string, body interface{}) error {
	req := &batchRequest{
		ctx: ctx,
		exec: func(ctx context.Context) error {
			return b.client.do(ctx, method, path, body, nil, nil)
		},
	}
	select {
	case b.queue <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Flush waits for pending requests to be dispatched.
func (b *Batcher) Flush(ctx context.Context) error {
	ack := make(chan error, 1)
	select {
	case b.flushCh <- ack:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-ack:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop terminates the batcher.
func (b *Batcher) Stop() {
	b.once.Do(func() {
		close(b.stopCh)
		<-b.doneCh
	})
}

func (b *Batcher) run() {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	defer close(b.doneCh)
	var batch []*batchRequest
	flush := func() {
		if len(batch) == 0 {
			return
		}
		for _, req := range batch {
			_ = req.exec(req.ctx)
		}
		batch = batch[:0]
	}
	for {
		select {
		case req := <-b.queue:
			if req == nil {
				flush()
				return
			}
			batch = append(batch, req)
			if len(batch) >= b.batchSize {
				flush()
			}
		case ack := <-b.flushCh:
			b.collect(&batch)
			flush()
			ack <- nil
		case <-ticker.C:
			flush()
		case <-b.stopCh:
			flush()
			return
		}
	}
}
