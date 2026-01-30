package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Formatter serializes arbitrary structures.
type Formatter interface {
	Format(v interface{}) ([]byte, error)
}

// NewFormatter returns the requested formatter.
func NewFormatter(kind string) Formatter {
	switch strings.ToLower(kind) {
	case "table":
		return TableFormatter{}
	case "yaml":
		return YAMLFormatter{}
	default:
		return JSONFormatter{}
	}
}

// JSONFormatter emits indented JSON.
type JSONFormatter struct{}

func (JSONFormatter) Format(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// YAMLFormatter emits YAML documents.
type YAMLFormatter struct{}

func (YAMLFormatter) Format(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// TableFormatter renders key/value pairs as a simple table.
type TableFormatter struct{}

func (TableFormatter) Format(v interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	switch data := v.(type) {
	case map[string]string:
		for k, val := range data {
			fmt.Fprintf(w, "%s\t%s\n", k, val)
		}
	case map[string]interface{}:
		for k, val := range data {
			fmt.Fprintf(w, "%s\t%v\n", k, val)
		}
	default:
		fmt.Fprintf(w, "%v\n", v)
	}
	w.Flush()
	return buf.Bytes(), nil
}
