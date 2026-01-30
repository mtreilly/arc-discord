package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/yourorg/arc-sdk/output"
	arcer "github.com/yourorg/arc-sdk/errors"
)

type tableData struct {
	headers []string
	rows    [][]string
}

func renderOutput(cmd *cobra.Command, opts output.OutputOptions, data any, table *tableData) error {
	switch {
	case opts.Is(output.OutputQuiet):
		return nil
	case opts.Is(output.OutputJSON):
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case opts.Is(output.OutputYAML):
		enc := yaml.NewEncoder(cmd.OutOrStdout())
		defer enc.Close()
		return enc.Encode(data)
	case opts.Is(output.OutputTable):
		if table == nil {
			return &arcer.CLIError{Msg: "table output not supported for this command", Hint: "use --output json or --output yaml"}
		}
		return renderTable(cmd, table)
	default:
		return &arcer.CLIError{Msg: fmt.Sprintf("unsupported output format %q", opts.Format), Hint: "valid options: table|json|yaml|quiet"}
	}
}

func renderTable(cmd *cobra.Command, tbl *tableData) error {
	if tbl == nil {
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	if len(tbl.headers) > 0 {
		fmt.Fprintln(w, strings.Join(tbl.headers, "\t"))
	}
	for _, row := range tbl.rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

func keyValueTable(m map[string]string) *tableData {
	rows := make([][]string, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rows = append(rows, []string{k, m[k]})
	}
	return &tableData{headers: []string{"Field", "Value"}, rows: rows}
}
