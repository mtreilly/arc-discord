package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func BenchmarkClientSend(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(0))
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	msg := &types.WebhookMessage{Content: "benchmark payload"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := client.Send(ctx, msg); err != nil {
			b.Fatalf("Send() error: %v", err)
		}
	}
}
