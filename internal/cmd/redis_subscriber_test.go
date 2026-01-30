package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
)

type stubPubSub struct {
	messages [][]byte
	err      error
	closed   bool
}

func (s *stubPubSub) Close() error {
	s.closed = true
	return nil
}

func (s *stubPubSub) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	if len(s.messages) == 0 {
		return nil, s.err
	}
	payload := s.messages[0]
	s.messages = s.messages[1:]
	return &redis.Message{Payload: string(payload)}, nil
}

func TestRedisSubscriberHandlerCalled(t *testing.T) {
	stub := &stubPubSub{messages: [][]byte{[]byte("payload")}, err: context.Canceled}
	s := &redisSubscriber{
		subscribe: func(ctx context.Context, channel string) pubSub { return stub },
	}
	called := false
	err := s.Subscribe(context.Background(), func(ctx context.Context, b []byte) error {
		called = true
		if string(b) != "payload" {
			t.Fatalf("unexpected payload %s", b)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
	if !stub.closed {
		t.Fatalf("expected pubsub close")
	}
}

func TestRedisSubscriberPropagatesHandlerError(t *testing.T) {
	stub := &stubPubSub{messages: [][]byte{[]byte("payload")}}
	s := &redisSubscriber{subscribe: func(ctx context.Context, channel string) pubSub { return stub }}
	want := errors.New("boom")
	err := s.Subscribe(context.Background(), func(ctx context.Context, b []byte) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestRedisSubscriberReturnsOnContextCancel(t *testing.T) {
	stub := &stubPubSub{err: context.Canceled}
	s := &redisSubscriber{subscribe: func(ctx context.Context, channel string) pubSub { return stub }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Subscribe(ctx, nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
