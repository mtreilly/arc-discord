package interactions

import (
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestCommandBuilderAdvanced(t *testing.T) {
	builder := NewSlashCommand("moderate", "Moderation utilities").
		AddStringOption("mode", "Mode to apply", true).
		AddIntegerOption("count", "Number of items", false).
		AddBooleanOption("silent", "Suppress output", false).
		AddUserOption("target", "Target user", true).
		AddRoleOption("role", "Target role", false).
		AddChannelOption("channel", "Target channel", false, types.ChannelTypeGuildText).
		AddMentionableOption("any", "Mention target", false).
		AddNumberOption("ratio", "Adjustment ratio", false).
		AddAttachmentOption("proof", "Evidence attachment", false).
		AddChoices("mode", types.ApplicationCommandChoice{Name: "Auto", Value: "auto"})

	builder.SetDefaultPermission(true).
		SetDefaultMemberPermissions("0x0000000000000008").
		SetNSFW(true)

	builder.AddSubcommand("config", "Configure defaults", func(sub *SubcommandBuilder) {
		sub.AddStringOption("timezone", "Timezone", false).
			AddIntegerOption("limit", "Limit value", true).
			AddChoices("timezone", types.ApplicationCommandChoice{Name: "UTC", Value: "utc"})
	})

	builder.AddSubcommandGroup("admin", "Admin flows", func(group *SubcommandGroupBuilder) {
		group.AddSubcommand("ban", "Ban a user", func(sub *SubcommandBuilder) {
			sub.AddUserOption("user", "User to ban", true).
				AddBooleanOption("silent", "Silent ban", false)
		})
	})

	cmd, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if cmd.DefaultMemberPermissions == nil || *cmd.DefaultMemberPermissions != "0x0000000000000008" {
		t.Fatalf("expected default member permissions to be set")
	}
	if cmd.DMPermission == nil || !*cmd.DMPermission {
		t.Fatalf("expected DM permission to be true")
	}
	if !cmd.NSFW {
		t.Fatalf("expected NSFW to be true")
	}
	if len(cmd.Options) != 11 {
		t.Fatalf("expected 11 options (including subcommands), got %d", len(cmd.Options))
	}
	mode := findOptionByName(cmd.Options, "mode")
	if mode == nil || len(mode.Choices) != 1 {
		t.Fatalf("expected mode choices to be set")
	}
	var adminGroup *types.ApplicationCommandOption
	for i := range cmd.Options {
		if cmd.Options[i].Type == types.CommandOptionSubCommandGroup {
			adminGroup = &cmd.Options[i]
		}
	}
	if adminGroup == nil || len(adminGroup.Options) != 1 {
		t.Fatalf("expected admin group with one subcommand, got %+v", adminGroup)
	}
	if len(adminGroup.Options[0].Options) != 2 {
		t.Fatalf("expected nested options on subcommand")
	}
}

func TestCommandBuilderChoiceValidation(t *testing.T) {
	builder := NewSlashCommand("test", "desc").
		AddBooleanOption("flag", "Flag", false).
		AddChoices("missing", types.ApplicationCommandChoice{Name: "on", Value: true})

	if _, err := builder.Build(); err == nil {
		t.Fatalf("expected error for missing option")
	}

	builder = NewSlashCommand("test", "desc").
		AddBooleanOption("flag", "Flag", false).
		AddChoices("flag", types.ApplicationCommandChoice{Name: "on", Value: true})
	if _, err := builder.Build(); err == nil {
		t.Fatalf("expected error for invalid choice target")
	}
}

func TestSubcommandBuilderChoiceValidation(t *testing.T) {
	builder := NewSlashCommand("nested", "desc").
		AddSubcommand("config", "Configure", func(sub *SubcommandBuilder) {
			sub.AddBooleanOption("enabled", "Enabled", false).
				AddChoices("enabled", types.ApplicationCommandChoice{Name: "on", Value: true})
		})

	if _, err := builder.Build(); err == nil {
		t.Fatalf("expected error for invalid subcommand choice target")
	}
}
