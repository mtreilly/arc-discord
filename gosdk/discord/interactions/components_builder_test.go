package interactions

import (
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestButtonBuilder(t *testing.T) {
	btn, err := NewButton("ok", "OK", types.ButtonStylePrimary).
		SetEmoji(&types.Emoji{Name: "🤖"}).
		SetDisabled(false).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if btn.CustomID != "ok" || btn.Label != "OK" {
		t.Fatalf("unexpected button %+v", btn)
	}

	_, err = NewLinkButton("Site", "").Build()
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestSelectMenuBuilder(t *testing.T) {
	menu, err := NewSelectMenu("menu").
		SetPlaceholder("Choose").
		AddOption("One", "one", "", nil, false).
		AddOption("Two", "two", "", nil, false).
		SetMinMaxValues(1, 2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(menu.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(menu.Options))
	}

	builder := SelectMenuOfType("channels", types.ComponentTypeChannelSelect).
		AddOption("One", "one", "", nil, false)
	if _, err := builder.Build(); err == nil {
		t.Fatal("expected error for options on channel select")
	}
}

func TestTextInputBuilder(t *testing.T) {
	input, err := NewTextInput("feedback", "Feedback", types.TextInputStyleParagraph).
		SetPlaceholder("Share thoughts").
		SetLength(5, 200).
		SetValue("Hello world").
		SetRequired(true).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if input.MinLength != 5 || input.MaxLength != 200 {
		t.Fatalf("unexpected min/max %+v", input)
	}

	if _, err := NewTextInput("bad", "", types.TextInputStyleParagraph).Build(); err == nil {
		t.Fatal("expected error for missing label")
	}
}

func TestActionRowBuilder(t *testing.T) {
	button, _ := NewButton("ok", "OK", types.ButtonStylePrimary).Build()
	row, err := NewActionRow().AddComponent(button).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(row.Components) != 1 {
		t.Fatalf("expected row child, got %+v", row.Components)
	}
}
