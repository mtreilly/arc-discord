package permissions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

// Permission represents a Discord permission bitfield.
type Permission int64

const (
	PermissionCreateInstantInvite Permission = 1 << iota
	PermissionKickMembers
	PermissionBanMembers
	PermissionAdministrator
	PermissionManageChannels
	PermissionManageGuild
	PermissionAddReactions
	PermissionViewAuditLog
	PermissionPrioritySpeaker
	PermissionStream
	PermissionViewChannel
	PermissionSendMessages
	PermissionSendTTSMessages
	PermissionManageMessages
	PermissionEmbedLinks
	PermissionAttachFiles
	PermissionReadMessageHistory
	PermissionMentionEveryone
	PermissionUseExternalEmojis
	PermissionViewGuildInsights
	PermissionConnect
	PermissionSpeak
	PermissionMuteMembers
	PermissionDeafenMembers
	PermissionMoveMembers
	PermissionUseVAD
	PermissionChangeNickname
	PermissionManageNicknames
	PermissionManageRoles
	PermissionManageWebhooks
	PermissionManageEmojis
	PermissionUseSlashCommands
	PermissionRequestToSpeak
	PermissionManageEvents
	PermissionManageThreads
	PermissionCreatePublicThreads
	PermissionCreatePrivateThreads
	PermissionUseExternalStickers
	PermissionSendMessagesInThreads
	PermissionUseEmbeddedActivities
	PermissionModerateMembers
)

var (
	allPermissions = []Permission{
		PermissionCreateInstantInvite,
		PermissionKickMembers,
		PermissionBanMembers,
		PermissionAdministrator,
		PermissionManageChannels,
		PermissionManageGuild,
		PermissionAddReactions,
		PermissionViewAuditLog,
		PermissionPrioritySpeaker,
		PermissionStream,
		PermissionViewChannel,
		PermissionSendMessages,
		PermissionSendTTSMessages,
		PermissionManageMessages,
		PermissionEmbedLinks,
		PermissionAttachFiles,
		PermissionReadMessageHistory,
		PermissionMentionEveryone,
		PermissionUseExternalEmojis,
		PermissionViewGuildInsights,
		PermissionConnect,
		PermissionSpeak,
		PermissionMuteMembers,
		PermissionDeafenMembers,
		PermissionMoveMembers,
		PermissionUseVAD,
		PermissionChangeNickname,
		PermissionManageNicknames,
		PermissionManageRoles,
		PermissionManageWebhooks,
		PermissionManageEmojis,
		PermissionUseSlashCommands,
		PermissionRequestToSpeak,
		PermissionManageEvents,
		PermissionManageThreads,
		PermissionCreatePublicThreads,
		PermissionCreatePrivateThreads,
		PermissionUseExternalStickers,
		PermissionSendMessagesInThreads,
		PermissionUseEmbeddedActivities,
		PermissionModerateMembers,
	}
	permissionNames = map[Permission]string{}
)

func init() {
	for _, perm := range allPermissions {
		permissionNames[perm] = perm.name()
	}
}

func (p Permission) name() string {
	switch p {
	case PermissionCreateInstantInvite:
		return "CreateInstantInvite"
	case PermissionKickMembers:
		return "KickMembers"
	case PermissionBanMembers:
		return "BanMembers"
	case PermissionAdministrator:
		return "Administrator"
	case PermissionManageChannels:
		return "ManageChannels"
	case PermissionManageGuild:
		return "ManageGuild"
	case PermissionAddReactions:
		return "AddReactions"
	case PermissionViewAuditLog:
		return "ViewAuditLog"
	case PermissionPrioritySpeaker:
		return "PrioritySpeaker"
	case PermissionStream:
		return "Stream"
	case PermissionViewChannel:
		return "ViewChannel"
	case PermissionSendMessages:
		return "SendMessages"
	case PermissionSendTTSMessages:
		return "SendTTSMessages"
	case PermissionManageMessages:
		return "ManageMessages"
	case PermissionEmbedLinks:
		return "EmbedLinks"
	case PermissionAttachFiles:
		return "AttachFiles"
	case PermissionReadMessageHistory:
		return "ReadMessageHistory"
	case PermissionMentionEveryone:
		return "MentionEveryone"
	case PermissionUseExternalEmojis:
		return "UseExternalEmojis"
	case PermissionViewGuildInsights:
		return "ViewGuildInsights"
	case PermissionConnect:
		return "Connect"
	case PermissionSpeak:
		return "Speak"
	case PermissionMuteMembers:
		return "MuteMembers"
	case PermissionDeafenMembers:
		return "DeafenMembers"
	case PermissionMoveMembers:
		return "MoveMembers"
	case PermissionUseVAD:
		return "UseVAD"
	case PermissionChangeNickname:
		return "ChangeNickname"
	case PermissionManageNicknames:
		return "ManageNicknames"
	case PermissionManageRoles:
		return "ManageRoles"
	case PermissionManageWebhooks:
		return "ManageWebhooks"
	case PermissionManageEmojis:
		return "ManageEmojis"
	case PermissionUseSlashCommands:
		return "UseSlashCommands"
	case PermissionRequestToSpeak:
		return "RequestToSpeak"
	case PermissionManageEvents:
		return "ManageEvents"
	case PermissionManageThreads:
		return "ManageThreads"
	case PermissionCreatePublicThreads:
		return "CreatePublicThreads"
	case PermissionCreatePrivateThreads:
		return "CreatePrivateThreads"
	case PermissionUseExternalStickers:
		return "UseExternalStickers"
	case PermissionSendMessagesInThreads:
		return "SendMessagesInThreads"
	case PermissionUseEmbeddedActivities:
		return "UseEmbeddedActivities"
	case PermissionModerateMembers:
		return "ModerateMembers"
	default:
		return "Unknown"
	}
}

// AllPermissions returns the bitmask containing every permission.
func AllPermissions() Permission {
	var mask Permission
	for _, perm := range allPermissions {
		mask |= perm
	}
	return mask
}

// PermissionFromString parses a numeric permission string into a Permission value.
func PermissionFromString(value string) Permission {
	if value == "" {
		return 0
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return Permission(n)
}

// Has reports whether all bits in mask are present.
func (p Permission) Has(mask Permission) bool {
	if mask == 0 {
		return true
	}
	return p&mask == mask
}

// Add returns the union of the permissions.
func (p Permission) Add(mask Permission) Permission {
	return p | mask
}

// Remove clears the given permissions.
func (p Permission) Remove(mask Permission) Permission {
	return p &^ mask
}

// String returns the human-readable list of active permissions.
func (p Permission) String() string {
	if p == 0 {
		return "none"
	}
	names := []string{}
	for _, perm := range allPermissions {
		if p.Has(perm) {
			names = append(names, permissionNames[perm])
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(names, ", "))
}

// PermissionCalculator evaluates permissions for a member in a channel.
type PermissionCalculator struct {
	guild   *types.Guild
	channel *types.Channel
	member  *types.Member
}

// NewPermissionCalculator constructs a calculator.
func NewPermissionCalculator(guild *types.Guild, channel *types.Channel, member *types.Member) *PermissionCalculator {
	return &PermissionCalculator{
		guild:   guild,
		channel: channel,
		member:  member,
	}
}

// ComputeBasePermissions calculates the guild-level permissions for the member.
func (pc *PermissionCalculator) ComputeBasePermissions() Permission {
	if pc.guild == nil || pc.member == nil || pc.member.User == nil {
		return 0
	}
	if pc.guild.OwnerID != "" && pc.member.User.ID == pc.guild.OwnerID {
		return AllPermissions()
	}
	mask := Permission(0)
	if role := pc.roleByID(pc.guild.ID); role != nil {
		mask |= PermissionFromString(role.Permissions)
	}
	for _, id := range pc.member.Roles {
		if role := pc.roleByID(id); role != nil {
			mask |= PermissionFromString(role.Permissions)
		}
	}
	return mask
}

// ComputeOverwrites applies channel overwrites to the base permissions.
func (pc *PermissionCalculator) ComputeOverwrites() Permission {
	base := pc.ComputeBasePermissions()
	allow, deny := pc.channelOverwrites()
	return (base &^ deny) | allow
}

// Compute returns the effective permission for the member in the channel.
func (pc *PermissionCalculator) Compute() Permission {
	return pc.ComputeOverwrites()
}

// Can returns true if the effective permissions include the requested mask.
func (pc *PermissionCalculator) Can(mask Permission) bool {
	return pc.Compute().Has(mask)
}

// CanManageChannel reports whether the member can manage the current channel.
func (pc *PermissionCalculator) CanManageChannel() bool {
	return pc.Can(PermissionManageChannels)
}

// CanSendMessages reports whether the member can send messages.
func (pc *PermissionCalculator) CanSendMessages() bool {
	return pc.Can(PermissionSendMessages)
}

func (pc *PermissionCalculator) roleByID(id string) *types.Role {
	if pc.guild == nil {
		return nil
	}
	for i := range pc.guild.Roles {
		if pc.guild.Roles[i].ID == id {
			return &pc.guild.Roles[i]
		}
	}
	return nil
}

func (pc *PermissionCalculator) channelOverwrites() (Permission, Permission) {
	if pc.channel == nil || pc.member == nil || pc.member.User == nil {
		return 0, 0
	}
	var allow, deny Permission
	for _, overwrite := range pc.channel.PermissionOverwrites {
		permAllow, permDeny := parseOverwrite(overwrite)
		switch overwrite.Type {
		case types.PermissionOverwriteRole:
			if overwrite.ID == pc.guild.ID {
				allow |= permAllow
				deny |= permDeny
				continue
			}
			for _, roleID := range pc.member.Roles {
				if roleID == overwrite.ID {
					allow |= permAllow
					deny |= permDeny
				}
			}
		case types.PermissionOverwriteMember:
			if overwrite.ID == pc.member.User.ID {
				allow |= permAllow
				deny |= permDeny
			}
		}
	}
	return allow, deny
}

func parseOverwrite(overwrite types.PermissionOverwrite) (Permission, Permission) {
	return PermissionFromString(overwrite.Allow), PermissionFromString(overwrite.Deny)
}
