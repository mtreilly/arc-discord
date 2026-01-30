package types

import (
	"strings"
	"testing"
)

func TestInteractionValidate(t *testing.T) {
	inter := &Interaction{ID: "123", Token: "token"}
	if err := inter.Validate(); err != nil {
		t.Fatalf("expected valid interaction: %v", err)
	}

	inter.ID = ""
	if err := inter.Validate(); err == nil {
		t.Fatalf("expected error for missing ID")
	}
}

func TestApplicationCommandValidate(t *testing.T) {
	cmd := &ApplicationCommand{Name: "hello", Description: "desc"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected valid command: %v", err)
	}

	cmd.Name = ""
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected error for empty name")
	}
}

func TestApplicationCommandOptionValidate(t *testing.T) {
	opt := ApplicationCommandOption{
		Name:        "option",
		Description: "desc",
	}
	if err := opt.Validate(); err != nil {
		t.Fatalf("expected valid option: %v", err)
	}

	opt.Name = ""
	if err := opt.Validate(); err == nil {
		t.Fatalf("expected error for name")
	}
}

func TestInteractionResponseValidate_Message(t *testing.T) {
	resp := &InteractionResponse{
		Type: InteractionResponseChannelMessageWithSource,
		Data: &InteractionApplicationCommandCallbackData{
			Content: "hello world",
			Components: []MessageComponent{
				{
					Type: ComponentTypeActionRow,
					Components: []MessageComponent{
						{Type: ComponentTypeButton, CustomID: "btn_1", Label: "Run"},
					},
				},
			},
		},
	}
	if err := resp.Validate(); err != nil {
		t.Fatalf("expected valid response: %v", err)
	}

	resp.Data.Content = strings.Repeat("a", 2001)
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error for content length")
	}

	resp.Data.Content = "ok"
	resp.Data.Embeds = make([]Embed, 11)
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error for embed count")
	}
}

func TestInteractionResponseValidate_MessageComponents(t *testing.T) {
	resp := &InteractionResponse{
		Type: InteractionResponseChannelMessageWithSource,
		Data: &InteractionApplicationCommandCallbackData{
			Content: "hello",
			Components: []MessageComponent{
				{Type: ComponentTypeButton, CustomID: "invalid"},
			},
		},
	}
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error for missing action row")
	}
}

func TestInteractionResponseValidate_DataRequirement(t *testing.T) {
	resp := &InteractionResponse{Type: InteractionResponseChannelMessageWithSource}
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error when data is missing for message response")
	}
}

func TestInteractionResponseValidate_Autocomplete(t *testing.T) {
	resp := &InteractionResponse{
		Type: InteractionResponseAutocompleteResult,
		Data: &InteractionApplicationCommandCallbackData{
			Choices: []AutocompleteChoice{
				{Name: "One", Value: 1},
			},
		},
	}
	if err := resp.Validate(); err != nil {
		t.Fatalf("expected valid autocomplete response: %v", err)
	}

	resp.Data.Choices = nil
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error for missing choices")
	}

	resp.Data = &InteractionApplicationCommandCallbackData{
		Choices: []AutocompleteChoice{
			{Name: "One", Value: true},
		},
	}
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error for invalid choice value type")
	}
}

func TestInteractionResponseValidate_Modal(t *testing.T) {
	resp := &InteractionResponse{
		Type: InteractionResponseModal,
		Data: &InteractionApplicationCommandCallbackData{
			CustomID: "modal_1",
			Title:    "Example Modal",
			Components: []MessageComponent{
				{
					Type: ComponentTypeActionRow,
					Components: []MessageComponent{
						{Type: ComponentTypeTextInput, CustomID: "input_1", Label: "Label"},
					},
				},
			},
		},
	}
	if err := resp.Validate(); err != nil {
		t.Fatalf("expected valid modal response: %v", err)
	}

	resp.Data.Components[0].Components[0].Type = ComponentTypeButton
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error when modal child is not text input")
	}

	resp.Data.Components[0].Components[0].Type = ComponentTypeTextInput
	resp.Data.Components[0].Components = append(resp.Data.Components[0].Components, MessageComponent{
		Type: ComponentTypeTextInput, CustomID: "input_2", Label: "Another",
	})
	if err := resp.Validate(); err == nil {
		t.Fatal("expected error when modal action row has multiple text inputs")
	}
}
