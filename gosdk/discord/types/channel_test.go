package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestChannelValidate(t *testing.T) {
	ch := &Channel{Name: "general"}
	if err := ch.Validate(); err != nil {
		t.Fatalf("expected valid channel, got %v", err)
	}

	ch.Name = strings.Repeat("x", 101)
	if err := ch.Validate(); err == nil {
		t.Fatal("expected error for long name")
	}
}

func TestChannelCreateParamsValidate(t *testing.T) {
	params := &ChannelCreateParams{
		Name:             "build-updates",
		Type:             ChannelTypeGuildText,
		Topic:            "Build notifications",
		RateLimitPerUser: 5,
	}

	if err := params.Validate(); err != nil {
		t.Fatalf("expected params to validate: %v", err)
	}

	params.RateLimitPerUser = 999999
	if err := params.Validate(); err == nil {
		t.Fatal("expected rate limit validation error")
	}
}

func TestChannelParamsBuilder(t *testing.T) {
	builder := NewChannelParamsBuilder("deployments", ChannelTypeGuildText).
		Topic("Deployment feed").
		NSFW(false).
		Parent("1234")

	params, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if params.Name != "deployments" || params.ParentID != "1234" {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestModifyChannelParamsValidate(t *testing.T) {
	params := &ModifyChannelParams{
		Name:             "updates",
		Topic:            "Release info",
		RateLimitPerUser: 10,
	}
	if err := params.Validate(); err != nil {
		t.Fatalf("expected valid params, got %v", err)
	}

	params.Name = strings.Repeat("a", 101)
	if err := params.Validate(); err == nil {
		t.Fatal("expected error for long name")
	}
}

func TestChannelJSONMarshalling(t *testing.T) {
	now := time.Now().UTC()
	ch := &Channel{
		ID:               "42",
		Name:             "events",
		Type:             ChannelTypeGuildText,
		LastPinTimestamp: &now,
		PermissionOverwrites: []PermissionOverwrite{
			{ID: "role", Type: PermissionOverwriteRole, Allow: "123", Deny: "0"},
		},
	}

	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if !strings.Contains(string(data), `"events"`) {
		t.Fatalf("expected JSON to contain channel name, got %s", data)
	}
}
