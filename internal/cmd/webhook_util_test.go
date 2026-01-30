package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestCollectAttachmentSpecs(t *testing.T) {
	specs, err := collectAttachmentSpecs([]string{"/tmp/file.txt:renamed.txt"}, []string{"/tmp/hidden.bin"})
	if err != nil {
		t.Fatalf("collectAttachmentSpecs error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].name != "renamed.txt" || specs[0].spoiler {
		t.Fatalf("unexpected first spec: %#v", specs[0])
	}
	if specs[1].name != "SPOILER_hidden.bin" || !specs[1].spoiler {
		t.Fatalf("unexpected second spec: %#v", specs[1])
	}
}

func TestPrepareAttachments(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	specs := []attachmentSpec{{path: filePath, name: "sample.txt"}}
	attachments, cleanup, err := prepareAttachments(specs)
	if err != nil {
		t.Fatalf("prepareAttachments error: %v", err)
	}
	defer cleanup()

	if len(attachments) != 1 {
		t.Fatalf("expected attachment, got %d", len(attachments))
	}
	if attachments[0].Name != "sample.txt" {
		t.Fatalf("unexpected name: %s", attachments[0].Name)
	}
}

func TestLoadEmbeds(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "embed.json")
	data := `{"title":"Alerts"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write embed: %v", err)
	}

	embeds, err := loadEmbeds([]string{path})
	if err != nil {
		t.Fatalf("loadEmbeds error: %v", err)
	}
	if len(embeds) != 1 || embeds[0].Title != "Alerts" {
		t.Fatalf("unexpected embeds: %#v", embeds)
	}
}

func TestLoadComponents(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "components.json")
	data := `{"type":1,"components":[{"type":2,"label":"OK","style":1,"custom_id":"ok"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write components: %v", err)
	}

	comps, err := loadComponents([]string{path})
	if err != nil {
		t.Fatalf("loadComponents error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected one component, got %d", len(comps))
	}
	if comps[0].Type != types.ComponentTypeActionRow {
		t.Fatalf("expected action row, got type %d", comps[0].Type)
	}
	if len(comps[0].Components) != 1 {
		t.Fatalf("expected nested component, got %d", len(comps[0].Components))
	}
}
