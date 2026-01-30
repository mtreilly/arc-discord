package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/logger"
)

func TestRecommendedShardingUsesRecommendedCount(t *testing.T) {
	rs := &RecommendedSharding{}
	rs.SetRecommended(5)
	if count := rs.Calculate(1000); count != 5 {
		t.Fatalf("expected recommended count 5, got %d", count)
	}
}

func TestAutoScaleUpdatesShardCount(t *testing.T) {
	info := GatewayBotInfo{URL: "wss://example", Shards: 3}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot token" {
			t.Fatalf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	sm := NewShardManager("token", 1, 0,
		WithShardGatewayBotURL(server.URL),
		WithShardGatewayHTTPClient(server.Client()),
		WithShardLogger(logger.Default()),
	)
	if err := sm.AutoScale(context.Background(), 100, &RecommendedSharding{}); err != nil {
		t.Fatalf("auto scale error: %v", err)
	}
	if sm.shardCount != 3 {
		t.Fatalf("expected shard count 3, got %d", sm.shardCount)
	}
}

func TestFetchGatewayBotInfoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := fetchGatewayBotInfo(context.Background(), server.Client(), server.URL, "token")
	if err == nil {
		t.Fatalf("expected error for non-200 response")
	}
}
