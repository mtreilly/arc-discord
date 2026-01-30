package embeds

import (
	"strings"
	"testing"
	"time"
)

func TestBuilderBuildsValidEmbed(t *testing.T) {
	embed, err := New().
		SetTitle("Hello").
		SetDescription("World").
		SetColor(0x00FF00).
		SetURL("https://example.com").
		SetTimestamp(time.Now()).
		SetFooter("Foot", "https://icon").
		AddField("Field1", "Value1", true).
		AddField("Field2", "Value2", false).
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embed.Title != "Hello" || len(embed.Fields) != 2 {
		t.Fatalf("unexpected embed %+v", embed)
	}
}

func TestBuilderValidatesLimits(t *testing.T) {
	long := ""
	for i := 0; i < maxTitleRunes+1; i++ {
		long += "a"
	}
	if _, err := New().SetTitle(long).Build(); err == nil {
		t.Fatalf("expected error for long title")
	}

	longValue := strings.Repeat("b", maxFieldValueRunes+1)
	if _, err := New().AddField("Name", longValue, false).Build(); err == nil {
		t.Fatalf("expected error for long field value")
	}

	builder := New()
	for i := 0; i < maxFields; i++ {
		builder.AddField("n", "v", false)
	}
	if _, err := builder.AddField("overflow", "v", false).Build(); err == nil {
		t.Fatalf("expected error for field overflow")
	}
}
