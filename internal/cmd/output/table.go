package output

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
)

// Table collects headers, rows, and an optional footer, then renders
// them using olekukonko/tablewriter.
type Table struct {
	headers []string
	rows    [][]string
	footer  string
}

// NewTable creates a new Table with the given column headers.
func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
	}
}

// AddRow appends a row to the table.
func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

// SetFooter sets a summary line displayed below the table.
func (t *Table) SetFooter(format string, args ...interface{}) {
	t.footer = fmt.Sprintf(format, args...)
}

// Render writes the table to w.
func (t *Table) Render(w io.Writer) {
	tw := tablewriter.NewTable(w)
	tw.Header(toAny(t.headers)...)
	for _, row := range t.rows {
		_ = tw.Append(toAny(row)...)
	}
	_ = tw.Render()

	if t.footer != "" {
		fmt.Fprintf(w, "\n%s\n", t.footer)
	}
}

// toAny converts []string to []any for tablewriter's variadic API.
func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
