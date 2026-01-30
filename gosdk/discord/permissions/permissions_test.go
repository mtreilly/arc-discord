package permissions

import (
	"fmt"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestPermissionMaskOperations(t *testing.T) {
	base := PermissionSendMessages.Add(PermissionEmbedLinks)
	if !base.Has(PermissionSendMessages) || !base.Has(PermissionEmbedLinks) {
		t.Fatalf("missing permission bits")
	}
	staged := base.Remove(PermissionEmbedLinks)
	if staged.Has(PermissionEmbedLinks) {
		t.Fatalf("failed to remove permission")
	}
	if staged.String() == "" {
		t.Fatalf("string representation should not be empty")
	}
}

func TestPermissionCalculatorBase(t *testing.T) {
	guild := &types.Guild{
		ID:      "g1",
		OwnerID: "u1",
		Roles: []types.Role{
			{ID: "g1", Permissions: fmt.Sprintf("%d", PermissionViewChannel)},
			{ID: "r1", Permissions: fmt.Sprintf("%d", PermissionSendMessages)},
		},
	}
	member := &types.Member{User: &types.User{ID: "u1"}, Roles: []string{"r1"}}
	calculator := NewPermissionCalculator(guild, nil, member)
	if !calculator.Can(AllPermissions()) {
		t.Fatalf("owner should have all permissions")
	}
}

func TestPermissionCalculatorChannelOverrides(t *testing.T) {
	guild := &types.Guild{
		ID:      "g1",
		OwnerID: "u2",
		Roles: []types.Role{
			{ID: "g1", Permissions: "1"},
			{ID: "r1", Permissions: "1024"},
		},
	}
	channel := &types.Channel{
		PermissionOverwrites: []types.PermissionOverwrite{
			{ID: "r1", Type: types.PermissionOverwriteRole, Allow: fmt.Sprintf("%d", PermissionManageMessages), Deny: fmt.Sprintf("%d", PermissionManageChannels)},
			{ID: "u3", Type: types.PermissionOverwriteMember, Allow: fmt.Sprintf("%d", PermissionMentionEveryone), Deny: "0"},
		},
	}
	member := &types.Member{User: &types.User{ID: "u3"}, Roles: []string{"r1"}}
	calculator := NewPermissionCalculator(guild, channel, member)
	effective := calculator.Compute()
	if !effective.Has(PermissionManageMessages) {
		t.Fatalf("role allow should grant manage messages")
	}
	if effective.Has(PermissionManageChannels) {
		t.Fatalf("deny should block manage channels")
	}
}
