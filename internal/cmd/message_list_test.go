package cmd

import (
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestFilterMessages(t *testing.T) {
	msgs := []*types.Message{
		{ID: "1", Content: "Deploy complete", Author: &types.User{ID: "u1", Username: "bot"}},
		{ID: "2", Content: "Error stack", Author: &types.User{ID: "u2", Username: "ops"}},
		{ID: "3", Content: "deploy rolled back", Author: &types.User{ID: "u2", Username: "ops"}},
	}

	filtered := filterMessages(msgs, "deploy", "")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(filtered))
	}

	filtered = filterMessages(msgs, "deploy", "u2")
	if len(filtered) != 1 || filtered[0].ID != "3" {
		t.Fatalf("expected only message 3, got %#v", filtered)
	}

	filtered = filterMessages(msgs, "", "u1")
	if len(filtered) != 1 || filtered[0].ID != "1" {
		t.Fatalf("expected author filter to return message 1")
	}
}
