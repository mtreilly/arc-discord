package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestGuildsGetGuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "with_counts=true" {
			t.Fatalf("expected with_counts query, got %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(types.Guild{ID: "1", Name: "Guild"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	guild, err := client.Guilds().GetGuild(context.Background(), "1", true)
	if err != nil {
		t.Fatalf("GetGuild error: %v", err)
	}
	if guild.ID != "1" {
		t.Fatalf("expected guild ID 1, got %s", guild.ID)
	}
}

func TestGuildsModifyGuild(t *testing.T) {
	var reason string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reason = r.Header.Get("X-Audit-Log-Reason")
		json.NewEncoder(w).Encode(types.Guild{ID: "1", Name: "Updated"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	params := &types.GuildModifyParams{Name: "Updated", AuditLogReason: "cleanup"}
	guild, err := client.Guilds().ModifyGuild(context.Background(), "1", params)
	if err != nil {
		t.Fatalf("ModifyGuild error: %v", err)
	}
	if guild.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if reason == "" {
		t.Fatalf("expected audit log reason header")
	}
}

func TestGuildsCreateChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(types.Channel{ID: "10", Name: "new"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	channel, err := client.Guilds().CreateGuildChannel(context.Background(), "1", &types.ChannelCreateParams{Name: "new"})
	if err != nil {
		t.Fatalf("CreateGuildChannel error: %v", err)
	}
	if channel.ID != "10" {
		t.Fatalf("expected channel ID 10")
	}
}

func TestGuildRoleOperations(t *testing.T) {
	var recordedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedPath = r.URL.Path
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]*types.Role{{ID: "1", Name: "Admin"}})
		case http.MethodPost, http.MethodPatch:
			json.NewEncoder(w).Encode(types.Role{ID: "1", Name: "Admin"})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	roles, err := client.Guilds().GetGuildRoles(context.Background(), "1")
	if err != nil || len(roles) != 1 {
		t.Fatalf("GetGuildRoles error: %v", err)
	}

	role, err := client.Guilds().CreateGuildRole(context.Background(), "1", &types.RoleCreateParams{Name: "Admin"})
	if err != nil || role == nil {
		t.Fatalf("CreateGuildRole error: %v", err)
	}

	if _, err := client.Guilds().ModifyGuildRole(context.Background(), "1", "2", &types.RoleModifyParams{}); err != nil {
		t.Fatalf("ModifyGuildRole error: %v", err)
	}

	if err := client.Guilds().DeleteGuildRole(context.Background(), "1", "2"); err != nil {
		t.Fatalf("DeleteGuildRole error: %v", err)
	}

	if recordedPath == "" {
		t.Fatalf("expected requests to hit server")
	}
}

func TestGuildMemberOperations(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			if r.URL.Path == "/guilds/1/members" {
				json.NewEncoder(w).Encode([]*types.Member{{Roles: []string{"1"}}})
				return
			}
			json.NewEncoder(w).Encode(types.Member{Roles: []string{"1"}})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if _, err := client.Guilds().GetGuildMember(context.Background(), "1", "42"); err != nil {
		t.Fatalf("GetGuildMember error: %v", err)
	}
	if _, err := client.Guilds().ListGuildMembers(context.Background(), "1", &types.ListMembersParams{Limit: 10}); err != nil {
		t.Fatalf("ListGuildMembers error: %v", err)
	}
	if err := client.Guilds().AddGuildMemberRole(context.Background(), "1", "42", "99"); err != nil {
		t.Fatalf("AddGuildMemberRole error: %v", err)
	}
	if err := client.Guilds().RemoveGuildMemberRole(context.Background(), "1", "42", "99"); err != nil {
		t.Fatalf("RemoveGuildMemberRole error: %v", err)
	}
	if len(requests) == 0 {
		t.Fatalf("expected requests")
	}
}
