package interactions

import (
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestResponseBuilder_Message(t *testing.T) {
	btn, _ := NewButton("run", "Run", types.ButtonStylePrimary).Build()
	row, _ := NewActionRow().AddComponent(btn).Build()

	builder := NewMessageResponse("hello").
		SetContent("updated").
		SetTTS(true).
		SetAllowedMentions(&types.AllowedMentions{RepliedUser: true}).
		AddEmbed(types.Embed{Title: "Embed"}).
		AddComponentRow(row).
		SetEphemeral(true)

	resp, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if resp.Type != types.InteractionResponseChannelMessageWithSource {
		t.Fatalf("unexpected response type %d", resp.Type)
	}
	if resp.Data == nil || resp.Data.Content != "updated" {
		t.Fatalf("expected updated content, got %+v", resp.Data)
	}
	if resp.Data.Flags&interactionResponseFlagEphemeral == 0 {
		t.Fatalf("expected ephemeral flag to be set")
	}
	if len(resp.Data.Components) != 1 {
		t.Fatalf("expected component row")
	}
}

func TestResponseBuilder_Modal(t *testing.T) {
	input, _ := NewTextInput("input", "Label", types.TextInputStyleShort).Build()
	row, _ := NewActionRow().AddComponent(input).Build()

	modal := NewModalResponse("modal", "Title")
	modal.SetModalComponents(row)

	resp, err := modal.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if resp.Type != types.InteractionResponseModal {
		t.Fatalf("expected modal type, got %d", resp.Type)
	}
}

func TestResponseBuilder_ComponentValidation(t *testing.T) {
	builder := NewMessageResponse("hello")
	builder.AddComponentRow(&types.Button{Style: types.ButtonStylePrimary})

	if _, err := builder.Build(); err == nil {
		t.Fatalf("expected error for non action row")
	}

	modal := NewModalResponse("modal", "Title")
	modal.SetModalComponents(&types.ActionRow{
		Components: []types.Component{
			&types.Button{Style: types.ButtonStylePrimary, CustomID: "btn", Label: "Btn"},
		},
	})
	if _, err := modal.Build(); err == nil {
		t.Fatalf("expected error for modal child that is not text input")
	}
}
