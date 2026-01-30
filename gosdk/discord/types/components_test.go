package types

import "testing"

func TestButtonValidate(t *testing.T) {
	btn := &Button{
		Style:    ButtonStylePrimary,
		Label:    "Click",
		CustomID: "click_me",
	}
	if err := btn.Validate(); err != nil {
		t.Fatalf("expected valid button, got %v", err)
	}

	btn.Style = ButtonStyleLink
	btn.URL = "https://example.com"
	btn.CustomID = ""
	if err := btn.Validate(); err != nil {
		t.Fatalf("expected valid link button, got %v", err)
	}

	btn.URL = ""
	if err := btn.Validate(); err == nil {
		t.Fatal("expected error for missing URL on link button")
	}
}

func TestSelectMenuValidate(t *testing.T) {
	menu := &SelectMenu{
		Type:     ComponentTypeSelectMenu,
		CustomID: "choices",
		Options: []SelectOption{
			{Label: "One", Value: "one"},
		},
		MinValues: 1,
		MaxValues: 1,
	}
	if err := menu.Validate(); err != nil {
		t.Fatalf("expected valid select menu, got %v", err)
	}

	menu.Options = nil
	if err := menu.Validate(); err == nil {
		t.Fatal("expected error for missing options")
	}
}

func TestTextInputValidate(t *testing.T) {
	input := &TextInput{
		CustomID:  "feedback",
		Label:     "Feedback",
		Style:     TextInputStyleParagraph,
		MinLength: 10,
		MaxLength: 100,
		Value:     "This is a test value",
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("expected valid text input, got %v", err)
	}

	input.Value = "short"
	if err := input.Validate(); err == nil {
		t.Fatal("expected error for value shorter than min_length")
	}
}

func TestActionRowToMessageComponent(t *testing.T) {
	row := &ActionRow{
		Components: []Component{
			&Button{Style: ButtonStylePrimary, Label: "Click", CustomID: "btn"},
		},
	}

	mc, err := row.ToMessageComponent()
	if err != nil {
		t.Fatalf("expected action row to convert, got %v", err)
	}
	if mc.Type != ComponentTypeActionRow {
		t.Fatalf("expected action row type, got %d", mc.Type)
	}
	if len(mc.Components) != 1 || mc.Components[0].Type != ComponentTypeButton {
		t.Fatalf("expected embedded button, got %+v", mc.Components)
	}
}
