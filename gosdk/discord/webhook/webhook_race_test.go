package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestClientSendConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(1))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			msg := &types.WebhookMessage{Content: t.Name()}
			if err := client.Send(context.Background(), msg); err != nil {
				t.Errorf("goroutine %d Send() error: %v", i, err)
			}
		}()
	}

	wg.Wait()
}
