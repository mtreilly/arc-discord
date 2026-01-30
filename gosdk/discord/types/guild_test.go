package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGuildValidate(t *testing.T) {
	guild := &Guild{ID: "123", Name: "Test"}
	if err := guild.Validate(); err != nil {
		t.Fatalf("expected valid guild, got %v", err)
	}

	guild.ID = ""
	if err := guild.Validate(); err == nil {
		t.Fatalf("expected error for missing ID")
	}
}

func TestRoleValidate(t *testing.T) {
	role := &Role{Name: "Admin"}
	if err := role.Validate(); err != nil {
		t.Fatalf("expected role valid, got %v", err)
	}

	role.Name = ""
	if err := role.Validate(); err == nil {
		t.Fatalf("expected error for missing role name")
	}
}

func TestGuildModifyParamsValidate(t *testing.T) {
	params := &GuildModifyParams{Name: "Valid"}
	if err := params.Validate(); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}

	params.Name = strings.Repeat("a", 101)
	if err := params.Validate(); err == nil {
		t.Fatalf("expected error for long name")
	}
}

func TestRoleParamValidation(t *testing.T) {
	create := &RoleCreateParams{Name: "Admin"}
	if err := create.Validate(); err != nil {
		t.Fatalf("expected create params valid: %v", err)
	}
	create.Name = ""
	if err := create.Validate(); err == nil {
		t.Fatal("expected create validation error")
	}

	modify := &RoleModifyParams{Name: "Helper"}
	if err := modify.Validate(); err != nil {
		t.Fatalf("expected modify params valid: %v", err)
	}
}

func TestListMembersParamsValidate(t *testing.T) {
	if err := (&ListMembersParams{Limit: 100}).Validate(); err != nil {
		t.Fatalf("expected valid params: %v", err)
	}
	if err := (&ListMembersParams{Limit: 2000}).Validate(); err == nil {
		t.Fatal("expected error for high limit")
	}
}

func TestGuildJSONMarshaling(t *testing.T) {
	now := time.Now().UTC()
	guild := &Guild{
		ID:      "1",
		Name:    "Guild",
		Members: []Member{{User: &User{ID: "u1"}, JoinedAt: now}},
	}

	data, err := json.Marshal(guild)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if len(data) == 0 || data[0] != '{' {
		t.Fatalf("unexpected JSON: %s", data)
	}
}
