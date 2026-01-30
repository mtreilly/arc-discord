package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/client"
)

func TestCheckerReportSuccess(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gateway/bot" {
			json.NewEncoder(w).Encode(map[string]interface{}{"url": "wss://example"})
			return
		}
		http.NotFound(w, r)
	}))
	defer apiServer.Close()

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookServer.Close()

	apiClient, err := client.New("token", client.WithBaseURL(apiServer.URL))
	if err != nil {
		t.Fatalf("failed to create client %v", err)
	}

	checker := NewChecker(apiClient,
		WithHTTPClient(http.DefaultClient),
		WithGatewayURL(gatewayServer.URL),
	)

	report, err := checker.Report(context.Background(), webhookServer.URL)
	if err != nil {
		t.Fatalf("report error: %v", err)
	}
	if report.Status != "ok" {
		t.Fatalf("expected ok status, got %s", report.Status)
	}
}

func TestCheckerWebhookFailure(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"url": "wss://example"})
	}))
	defer apiServer.Close()

	apiClient, _ := client.New("token", client.WithBaseURL(apiServer.URL))
	checker := NewChecker(apiClient, WithGatewayURL(apiServer.URL))

	if err := checker.CheckWebhook(context.Background(), "https://bad.example"); err == nil {
		t.Fatalf("expected error for unreachable webhook")
	}
}
