package sftp

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// The kinds of file this plugin recognises. Everything else is "other":
// still catalogued, just with no columns.
const (
	kindCSV       = "csv"
	kindTSV       = "tsv"
	kindJSON      = "json"
	kindJSONLines = "jsonl"
	kindParquet   = "parquet"
	kindAvro      = "avro"
	kindOther     = "other"
)

// fileKind classifies a file by its extension.
func fileKind(name string) string {
	switch extension(name) {
	case "csv":
		return kindCSV
	case "tsv":
		return kindTSV
	case "json":
		return kindJSON
	case "jsonl", "ndjson":
		return kindJSONLines
	case "parquet":
		return kindParquet
	case "avro":
		return kindAvro
	default:
		return kindOther
	}
}

// isStructured reports whether a file holds structured data, which is
// what structured_only keeps.
func isStructured(name string) bool {
	return fileKind(name) != kindOther
}

// hasColumns reports whether this plugin can read a file's columns.
// Parquet and Avro are structured but need a reader this plugin does
// not carry.
func hasColumns(kind string) bool {
	switch kind {
	case kindCSV, kindTSV, kindJSON, kindJSONLines:
		return true
	default:
		return false
	}
}

// separator is the field separator for a delimited file kind.
func separator(kind string) rune {
	if kind == kindTSV {
		return '\t'
	}
	return ','
}

// dataMimeTypes are the types the OS mime database gets wrong or does
// not know, so the answer does not depend on the machine Marmot runs on.
var dataMimeTypes = map[string]string{
	"csv":     "text/csv",
	"tsv":     "text/tab-separated-values",
	"json":    "application/json",
	"jsonl":   "application/x-ndjson",
	"ndjson":  "application/x-ndjson",
	"parquet": "application/vnd.apache.parquet",
	"avro":    "application/avro",
}

// mimeType guesses a file's type from its extension. Reading the file to
// sniff it would cost a fetch per file, which a directory of millions
// cannot afford.
func mimeType(name string) string {
	if known, ok := dataMimeTypes[extension(name)]; ok {
		return known
	}
	return mime.TypeByExtension(path.Ext(name))
}

// The column types this plugin infers, named as OpenMetadata names them
// so a merged asset agrees with a catalog imported from there.
const (
	typeInt      = "INT"
	typeFloat    = "FLOAT"
	typeBoolean  = "BOOLEAN"
	typeDatetime = "DATETIME"
	typeString   = "STRING"
)

// countingReader records how many bytes were pulled from a file, which
// is how a read that hit the byte cap is told from one that reached the
// end of the file.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// delimited is the result of reading a csv or tsv file.
type delimited struct {
	Header []string
	// Rows holds the first keepRows data rows, for inference or preview.
	Rows [][]string
	// RowCount is every data row seen, which is the file's real row count
	// when Exact is true and a lower bound otherwise.
	RowCount int64
	// Exact is true when the whole file was read.
	Exact bool
}

// readDelimited parses a delimited file, keeping at most keepRows rows
// and never reading more than maxBytes from it.
func readDelimited(r io.Reader, sep rune, maxBytes int64, keepRows int) (*delimited, error) {
	// One byte past the cap is read on purpose: seeing it is what proves
	// the file did not end there.
	counter := &countingReader{r: io.LimitReader(r, maxBytes+1)}

	reader := csv.NewReader(counter)
	reader.Comma = sep
	reader.LazyQuotes = true
	// Rows in a hand-maintained export are often ragged; a short or long
	// row should not abandon the file.
	reader.FieldsPerRecord = -1

	out := &delimited{}
	parsed := true

	for {
		record, err := reader.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				if out.Header == nil {
					return nil, fmt.Errorf("parsing delimited file: %w", err)
				}
				parsed = false
			}
			break
		}

		if out.Header == nil {
			out.Header = cleanHeader(record)
			continue
		}

		out.RowCount++
		if len(out.Rows) < keepRows {
			out.Rows = append(out.Rows, record)
		}
	}

	cappedByBytes := counter.n > maxBytes
	if cappedByBytes && out.RowCount > 0 {
		// The cap can cut a row in half, and half a row is not data.
		out.RowCount--
		if int64(len(out.Rows)) > out.RowCount {
			out.Rows = out.Rows[:len(out.Rows)-1]
		}
	}

	out.Exact = parsed && !cappedByBytes
	return out, nil
}

// cleanHeader tidies the header row: a byte order mark on the first cell
// is invisible in a spreadsheet but would show up in a column name, and
// an unnamed column still needs something to be called.
func cleanHeader(record []string) []string {
	header := make([]string, len(record))
	for i, name := range record {
		if i == 0 {
			name = strings.TrimPrefix(name, "\ufeff")
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		header[i] = name
	}
	return header
}

// columnKinds tracks which types every value in a column has satisfied
// so far. A kind stays possible only while no value has ruled it out.
type columnKinds struct {
	sawValue bool
	sawEmpty bool
	integer  bool
	float    bool
	boolean  bool
	datetime bool
}

func (k columnKinds) dataType() string {
	switch {
	case !k.sawValue:
		return typeString
	case k.integer:
		return typeInt
	case k.float:
		return typeFloat
	case k.boolean:
		return typeBoolean
	case k.datetime:
		return typeDatetime
	default:
		return typeString
	}
}

// inferDelimitedColumns reads the sampled rows and decides each column's
// type. A column is nullable when any sampled row left it empty.
func inferDelimitedColumns(header []string, rows [][]string) []pluginsdk.Column {
	kinds := make([]columnKinds, len(header))
	for i := range kinds {
		kinds[i] = columnKinds{integer: true, float: true, boolean: true, datetime: true}
	}

	for _, row := range rows {
		for i := range header {
			if i >= len(row) {
				// A row shorter than the header leaves the rest empty.
				kinds[i].sawEmpty = true
				continue
			}

			value := strings.TrimSpace(row[i])
			if value == "" {
				kinds[i].sawEmpty = true
				continue
			}

			k := &kinds[i]
			k.sawValue = true
			k.integer = k.integer && isInteger(value)
			k.float = k.float && isFloat(value)
			k.boolean = k.boolean && isBoolean(value)
			k.datetime = k.datetime && isDatetime(value)
		}
	}

	columns := make([]pluginsdk.Column, 0, len(header))
	for i, name := range header {
		columns = append(columns, pluginsdk.Column{
			Name:     name,
			DataType: kinds[i].dataType(),
			Nullable: kinds[i].sawEmpty,
		})
	}
	return columns
}

func isInteger(value string) bool {
	_, err := strconv.ParseInt(value, 10, 64)
	return err == nil
}

func isFloat(value string) bool {
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func isBoolean(value string) bool {
	return strings.EqualFold(value, "true") || strings.EqualFold(value, "false")
}

// datetimeLayouts are the timestamp formats a data export usually uses.
var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02",
	"2006-01-02 15:04:05",
}

func isDatetime(value string) bool {
	for _, layout := range datetimeLayouts {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

// jsonColumn is a JSON key with the number of records that carried it,
// which is how a reader tells a field every record has from one only a
// few do.
type jsonColumn struct {
	pluginsdk.Column
	Occurrence int `json:"occurrence"`
}

// jsonData is the result of reading a JSON or JSON-lines file.
type jsonData struct {
	// Records holds the first keepRecords objects, for inference or preview.
	Records []map[string]any
	// Count is every object seen within the byte cap.
	Count int64
	Exact bool
}

// readJSON reads objects from a file holding either one JSON object per
// line or a single top-level array of objects, which are the two shapes
// data lands on an SFTP server in.
func readJSON(r io.Reader, maxBytes int64, keepRecords int) (*jsonData, error) {
	counter := &countingReader{r: io.LimitReader(r, maxBytes+1)}
	buffered := bufio.NewReader(counter)

	start, err := peekJSONStart(buffered)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &jsonData{Exact: true}, nil
		}
		return nil, fmt.Errorf("reading json file: %w", err)
	}

	decoder := json.NewDecoder(buffered)
	inArray := start == '['
	if inArray {
		// Consume the opening bracket so Decode sees the elements.
		if _, err := decoder.Token(); err != nil {
			return nil, fmt.Errorf("reading json array: %w", err)
		}
	} else if start != '{' {
		return nil, fmt.Errorf("expected a json object or an array of objects, found %q", string(start))
	}

	out := &jsonData{}
	parsed := true

	for {
		if inArray && !decoder.More() {
			break
		}

		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			if !errors.Is(err, io.EOF) {
				if out.Count == 0 {
					return nil, fmt.Errorf("parsing json file: %w", err)
				}
				parsed = false
			}
			break
		}

		out.Count++
		if len(out.Records) < keepRecords {
			out.Records = append(out.Records, record)
		}
	}

	cappedByBytes := counter.n > maxBytes
	if cappedByBytes && out.Count > 0 {
		// The cap can cut an object in half, and half an object is not data.
		out.Count--
		if int64(len(out.Records)) > out.Count {
			out.Records = out.Records[:len(out.Records)-1]
		}
	}

	out.Exact = parsed && !cappedByBytes
	return out, nil
}

// peekJSONStart returns the first meaningful byte of the stream without
// consuming it, so the caller can tell an array from a stream of
// objects. A byte order mark is dropped: it is invisible to whoever
// wrote the file but would break the decoder.
func peekJSONStart(r *bufio.Reader) (byte, error) {
	if prefix, err := r.Peek(3); err == nil && string(prefix) == "\ufeff" {
		if _, err := r.Discard(3); err != nil {
			return 0, err
		}
	}

	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		if err := r.UnreadByte(); err != nil {
			return 0, err
		}
		return b, nil
	}
}

// jsonField accumulates what was seen for one key across records.
type jsonField struct {
	types      map[string]bool
	occurrence int
	sawNull    bool
}

// inferJSONColumns merges the sampled records into one column list: the
// top-level keys, plus one level of nesting written "parent.child".
// Keys are sorted, so a run over the same file always produces the same
// schema even though Go visits map keys in a random order.
func inferJSONColumns(records []map[string]any) []jsonColumn {
	fields := make(map[string]*jsonField)

	observe := func(name string, value any) {
		field, ok := fields[name]
		if !ok {
			field = &jsonField{types: make(map[string]bool)}
			fields[name] = field
		}
		field.occurrence++
		field.types[jsonType(value)] = true
		if value == nil {
			field.sawNull = true
		}
	}

	for _, record := range records {
		for key, value := range record {
			observe(key, value)

			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			for childKey, childValue := range nested {
				observe(key+"."+childKey, childValue)
			}
		}
	}

	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	columns := make([]jsonColumn, 0, len(names))
	for _, name := range names {
		field := fields[name]
		columns = append(columns, jsonColumn{
			Column: pluginsdk.Column{
				Name:     name,
				DataType: joinJSONTypes(field.types),
				// A key some records leave out is as optional as one they
				// set to null.
				Nullable: field.sawNull || field.occurrence < len(records),
			},
			Occurrence: field.occurrence,
		})
	}
	return columns
}

func jsonType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "string"
	}
}

// jsonTypeOrder fixes the order mixed types are written in, so
// "string|null" does not come out as "null|string" on the next run.
var jsonTypeOrder = []string{"string", "number", "boolean", "object", "array", "null"}

func joinJSONTypes(types map[string]bool) string {
	present := make([]string, 0, len(types))
	for _, name := range jsonTypeOrder {
		if types[name] {
			present = append(present, name)
		}
	}
	if len(present) == 0 {
		return "null"
	}
	return strings.Join(present, "|")
}

// jsonValue reads a column out of a record, following one level of
// nesting for a "parent.child" name.
func jsonValue(record map[string]any, column string) any {
	if value, ok := record[column]; ok {
		return value
	}

	parent, child, nested := strings.Cut(column, ".")
	if !nested {
		return nil
	}

	object, ok := record[parent].(map[string]any)
	if !ok {
		return nil
	}
	return object[child]
}
