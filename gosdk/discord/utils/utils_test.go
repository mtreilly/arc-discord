package utils

import (
	"testing"
	"time"
)

func TestParseMention(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<@123>", "123"},
		{"<@!456>", "456"},
		{"<#789>", "789"},
		{"<@&321>", "321"},
	}
	for _, tt := range tests {
		if val, ok := ParseMention(tt.input); !ok || val != tt.want {
			t.Fatalf("ParseMention(%s) = %s, ok=%v; want %s", tt.input, val, ok, tt.want)
		}
	}
}

func TestParseFormatEmoji(t *testing.T) {
	name, id, animated, ok := ParseEmoji("<:foo:123>")
	if !ok || name != "foo" || id != "123" || animated {
		t.Fatalf("unexpected parse %+v %+v %+v", name, id, animated)
	}
	formatted := FormatEmoji("bar", "456", true)
	if formatted != "<a:bar:456>" {
		t.Fatalf("expected animated output, got %s", formatted)
	}
}

func TestSnowflakeConversion(t *testing.T) {
	now := time.Now()
	sf := TimeToSnowflake(now)
	out, err := SnowflakeToTime(sf)
	if err != nil {
		t.Fatalf("SnowflakeToTime error: %v", err)
	}
	if out.Sub(now) > time.Second*5 {
		t.Fatalf("time mismatch: %v vs %v", out, now)
	}
}

func TestChunkSlice(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}
	chunks := ChunkSlice(data, 2)
	if len(chunks) != 3 {
		t.Fatalf("unexpected chunk count %d", len(chunks))
	}
}

func TestRateLimitDelay(t *testing.T) {
	reset := time.Now().Add(time.Second)
	delay := RateLimitDelay(1, 1, reset)
	if delay <= 0 {
		t.Fatalf("expected positive delay, got %v", delay)
	}
}
