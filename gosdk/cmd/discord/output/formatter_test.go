package output

import "testing"

func TestFormatters(t *testing.T) {
	data := map[string]string{"foo": "bar", "baz": "qux"}
	formats := []string{"json", "yaml", "table"}
	for _, kind := range formats {
		f := NewFormatter(kind)
		out, err := f.Format(data)
		if err != nil || len(out) == 0 {
			t.Fatalf("formatter %s failed: %v output=%s", kind, err, out)
		}
	}
}
