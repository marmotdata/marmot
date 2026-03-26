package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/yaml"
)

// Format represents the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Printer handles output formatting.
type Printer struct {
	format Format
	writer io.Writer
}

// NewPrinter creates a Printer for the given format string.
func NewPrinter(format string, writer io.Writer) *Printer {
	if writer == nil {
		writer = os.Stdout
	}

	f := FormatTable
	switch format {
	case "json":
		f = FormatJSON
	case "yaml":
		f = FormatYAML
	}

	return &Printer{
		format: f,
		writer: writer,
	}
}

// IsRaw returns true if the output format requires raw JSON/YAML.
func (p *Printer) IsRaw() bool {
	return p.format == FormatJSON || p.format == FormatYAML
}

// PrintRaw outputs raw bytes as JSON or YAML.
func (p *Printer) PrintRaw(data []byte) error {
	switch p.format {
	case FormatYAML:
		y, err := yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		_, err = p.writer.Write(y)
		return err
	default:
		// Pretty-print JSON
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			// If not valid JSON, print as-is
			_, err := p.writer.Write(data)
			return err
		}
		enc := json.NewEncoder(p.writer)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
}

// PrintJSON outputs a structured value as JSON or YAML.
func (p *Printer) PrintJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.PrintRaw(data)
}

// PrintTable prints a table using the given headers and rows.
func (p *Printer) PrintTable(t *Table) {
	t.Render(p.writer)
}

// PrintMessage prints a simple message (used across all formats).
func (p *Printer) PrintMessage(format string, args ...interface{}) {
	fmt.Fprintf(p.writer, format+"\n", args...)
}
