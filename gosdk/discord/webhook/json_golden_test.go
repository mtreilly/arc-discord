package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestWebhookMessageGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message *types.WebhookMessage
		golden  string
	}{
		{
			name: "simple full payload",
			message: &types.WebhookMessage{
				Content:   "Hello from golden test",
				Username:  "GoldenBot",
				AvatarURL: "https://example.com/avatar.png",
				TTS:       false,
				AllowedMentions: &struct {
					Parse []string `json:"parse,omitempty"`
				}{
					Parse: []string{"users"},
				},
				ThreadName: "golden-thread",
				Embeds: []types.Embed{
					{
						Title:       "Golden Embed",
						Description: "Golden embed body",
					},
				},
			},
			golden: "message_simple.json",
		},
		{
			name: "minimal payload",
			message: &types.WebhookMessage{
				Content: "Minimal payload",
			},
			golden: "message_minimal.json",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.message.Validate(); err != nil {
				t.Fatalf("message validation failed: %v", err)
			}

			got, err := json.MarshalIndent(tt.message, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal message: %v", err)
			}

			want := readGolden(t, tt.golden)

			compareJSON(t, tt.golden, want, got)
		})
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", "golden", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", name, err)
	}

	return data
}

func compareJSON(t *testing.T, name string, wantBytes, gotBytes []byte) {
	t.Helper()

	var wantData interface{}
	var gotData interface{}

	if err := json.Unmarshal(wantBytes, &wantData); err != nil {
		t.Fatalf("failed to unmarshal golden %s: %v", name, err)
	}
	if err := json.Unmarshal(gotBytes, &gotData); err != nil {
		t.Fatalf("failed to unmarshal got JSON: %v", err)
	}

	if !jsonDeepEqual(wantData, gotData) {
		t.Fatalf("golden mismatch for %s\nwant: %s\n got: %s", name, wantBytes, gotBytes)
	}
}

func jsonDeepEqual(a, b interface{}) bool {
	switch want := a.(type) {
	case map[string]interface{}:
		gotMap, ok := b.(map[string]interface{})
		if !ok || len(want) != len(gotMap) {
			return false
		}
		for k, v := range want {
			if !jsonDeepEqual(v, gotMap[k]) {
				return false
			}
		}
		return true
	case []interface{}:
		gotSlice, ok := b.([]interface{})
		if !ok || len(want) != len(gotSlice) {
			return false
		}
		for i := range want {
			if !jsonDeepEqual(want[i], gotSlice[i]) {
				return false
			}
		}
		return true
	default:
		return want == b
	}
}
