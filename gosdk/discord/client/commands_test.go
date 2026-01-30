package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestApplicationCommandsGetGlobalApplicationCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/applications/app123/commands" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]*types.ApplicationCommand{
			{ID: "cmd1", Name: "hello"},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	cmds, err := client.ApplicationCommands("app123").GetGlobalApplicationCommands(context.Background())
	if err != nil {
		t.Fatalf("GetGlobalApplicationCommands error: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != "cmd1" {
		t.Fatalf("unexpected commands %+v", cmds)
	}
}

func TestApplicationCommandsCreateGlobalApplicationCommand(t *testing.T) {
	var receivedReason string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/applications/app123/commands" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		receivedReason = r.Header.Get("X-Audit-Log-Reason")

		var payload types.ApplicationCommand
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Name != "greet" {
			t.Fatalf("expected name greet, got %s", payload.Name)
		}
		payload.ID = "cmd1"
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	cmd := &types.ApplicationCommand{
		Name:           "greet",
		Description:    "say hello",
		Type:           types.ApplicationCommandTypeChatInput,
		AuditLogReason: "initial sync",
	}

	created, err := client.ApplicationCommands("app123").CreateGlobalApplicationCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("CreateGlobalApplicationCommand error: %v", err)
	}
	if created.ID != "cmd1" {
		t.Fatalf("expected ID cmd1, got %s", created.ID)
	}
	if receivedReason != url.QueryEscape("initial sync") {
		t.Fatalf("expected encoded audit reason, got %s", receivedReason)
	}
}

func TestApplicationCommandsEditGlobalApplicationCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/applications/app123/commands/cmd1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var payload types.ApplicationCommand
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Description != "updated" {
			t.Fatalf("expected updated description, got %s", payload.Description)
		}
		payload.ID = "cmd1"
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	cmd := &types.ApplicationCommand{
		Name:        "greet",
		Description: "updated",
		Type:        types.ApplicationCommandTypeChatInput,
	}
	updated, err := client.ApplicationCommands("app123").EditGlobalApplicationCommand(context.Background(), "cmd1", cmd)
	if err != nil {
		t.Fatalf("EditGlobalApplicationCommand error: %v", err)
	}
	if updated.Description != "updated" {
		t.Fatalf("expected updated description, got %s", updated.Description)
	}
}

func TestApplicationCommandsDeleteGlobalApplicationCommand(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/applications/app123/commands/cmd1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.ApplicationCommands("app123").DeleteGlobalApplicationCommand(context.Background(), "cmd1"); err != nil {
		t.Fatalf("DeleteGlobalApplicationCommand error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestApplicationCommandsGuildOperations(t *testing.T) {
	var createCalled, editCalled, deleteCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/applications/app123/guilds/guild123/commands":
			_ = json.NewEncoder(w).Encode([]*types.ApplicationCommand{{ID: "cmd1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/applications/app123/guilds/guild123/commands":
			createCalled = true
			var payload types.ApplicationCommand
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			payload.ID = "cmd2"
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodPatch && r.URL.Path == "/applications/app123/guilds/guild123/commands/cmd2":
			editCalled = true
			var payload types.ApplicationCommand
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			payload.Description = "edited"
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodDelete && r.URL.Path == "/applications/app123/guilds/guild123/commands/cmd2":
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := newTestClient(t, server.URL).ApplicationCommands("app123")

	cmds, err := svc.GetGuildApplicationCommands(context.Background(), "guild123")
	if err != nil {
		t.Fatalf("GetGuildApplicationCommands error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	cmd := &types.ApplicationCommand{
		Name:        "local",
		Description: "guild command",
		Type:        types.ApplicationCommandTypeChatInput,
	}
	created, err := svc.CreateGuildApplicationCommand(context.Background(), "guild123", cmd)
	if err != nil {
		t.Fatalf("CreateGuildApplicationCommand error: %v", err)
	}
	if created.ID != "cmd2" || !createCalled {
		t.Fatalf("expected guild create to run")
	}

	if _, err := svc.EditGuildApplicationCommand(context.Background(), "guild123", "cmd2", cmd); err != nil {
		t.Fatalf("EditGuildApplicationCommand error: %v", err)
	}
	if !editCalled {
		t.Fatal("expected edit handler to run")
	}

	if err := svc.DeleteGuildApplicationCommand(context.Background(), "guild123", "cmd2"); err != nil {
		t.Fatalf("DeleteGuildApplicationCommand error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected delete handler to run")
	}
}

func TestApplicationCommandsBulkOverwrite(t *testing.T) {
	var globalCalled, guildCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/applications/app123/commands":
			globalCalled = true
		case "/applications/app123/guilds/guild123/commands":
			guildCalled = true
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) == 0 {
			t.Fatal("expected body bytes")
		}

		_ = json.NewEncoder(w).Encode([]*types.ApplicationCommand{{ID: "cmd1"}})
	}))
	defer server.Close()

	svc := newTestClient(t, server.URL).ApplicationCommands("app123")
	cmd := &types.ApplicationCommand{
		Name:        "bulk",
		Description: "bulk cmd",
		Type:        types.ApplicationCommandTypeChatInput,
	}

	if _, err := svc.BulkOverwriteGlobalApplicationCommands(context.Background(), []*types.ApplicationCommand{cmd}); err != nil {
		t.Fatalf("BulkOverwriteGlobalApplicationCommands error: %v", err)
	}
	if !globalCalled {
		t.Fatal("expected global overwrite call")
	}

	if _, err := svc.BulkOverwriteGuildApplicationCommands(context.Background(), "guild123", []*types.ApplicationCommand{cmd}); err != nil {
		t.Fatalf("BulkOverwriteGuildApplicationCommands error: %v", err)
	}
	if !guildCalled {
		t.Fatal("expected guild overwrite call")
	}
}

func TestApplicationCommandsBulkOverwriteNilSlice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload) != 0 {
			t.Fatalf("expected empty slice, got %+v", payload)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	svc := newTestClient(t, server.URL).ApplicationCommands("app123")
	if _, err := svc.BulkOverwriteGlobalApplicationCommands(context.Background(), nil); err != nil {
		t.Fatalf("BulkOverwriteGlobalApplicationCommands error: %v", err)
	}
}

func TestApplicationCommandsValidation(t *testing.T) {
	client := newTestClient(t, "http://example.com")
	svc := client.ApplicationCommands("")

	if _, err := svc.GetGlobalApplicationCommands(context.Background()); err == nil {
		t.Fatal("expected error for missing application ID")
	}

	if err := svc.DeleteGlobalApplicationCommand(context.Background(), ""); err == nil {
		t.Fatal("expected error for missing command ID")
	}

	invalidCmd := &types.ApplicationCommand{
		Name:        "",
		Description: "",
		Type:        types.ApplicationCommandTypeChatInput,
	}
	svc = client.ApplicationCommands("app123")
	if _, err := svc.CreateGlobalApplicationCommand(context.Background(), invalidCmd); err == nil {
		t.Fatal("expected validation error for invalid command")
	}

	if _, err := svc.BulkOverwriteGuildApplicationCommands(context.Background(), "", []*types.ApplicationCommand{}); err == nil {
		t.Fatal("expected error for missing guild ID")
	}
}
