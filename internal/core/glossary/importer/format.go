package importer

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Format string

const (
	FormatXLSX Format = "xlsx"
	FormatCSV  Format = "csv"
)

const (
	termsSheet  = "Terms"
	guideSheet  = "Guide"
	valuesSheet = "Values"
	// validationRows is how far down the template's drop-downs reach.
	validationRows = MaxRows + 1
)

var utf8BOM = []byte("\xef\xbb\xbf")

// Limits bound what an upload may cost to read.
const (
	MaxBytes = 5 << 20
	MaxRows  = 5000
	// maxUnzipped caps an XLSX's uncompressed size, so a zip bomb fails fast.
	maxUnzipped = 50 << 20
)

var (
	ErrUnsupportedFormat = errors.New("unsupported file format: use .xlsx or .csv")
	ErrTooLarge          = fmt.Errorf("file exceeds %d MB", MaxBytes>>20)
	ErrTooManyRows       = fmt.Errorf("file exceeds %d rows", MaxRows)
	ErrNoHeader          = errors.New("the file has no header row")
)

// FormatOf picks the format from an upload's name, then from its first bytes.
func FormatOf(filename string, head []byte) (Format, error) {
	switch {
	case strings.HasSuffix(strings.ToLower(filename), ".xlsx"):
		return FormatXLSX, nil
	case strings.HasSuffix(strings.ToLower(filename), ".csv"):
		return FormatCSV, nil
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		return FormatXLSX, nil
	}
	return "", ErrUnsupportedFormat
}

// Write renders the columns, and rows if any, as a template or an export. The
// XLSX carries the labels as cell comments, drop-downs for enum and boolean
// columns and a guide sheet; the CSV only the header and the rows.
func Write(w io.Writer, format Format, cols []Column, texts Texts, rows [][]string) error {
	switch format {
	case FormatCSV:
		return writeCSV(w, cols, rows)
	case FormatXLSX:
		return writeXLSX(w, cols, texts, rows)
	}
	return ErrUnsupportedFormat
}

func writeCSV(w io.Writer, cols []Column, rows [][]string) error {
	// The BOM makes Excel read the file as UTF-8.
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	out := csv.NewWriter(w)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = c.ID
	}
	if err := out.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		escaped := make([]string, len(row))
		for i, v := range row {
			escaped[i] = escapeFormula(v)
		}
		if err := out.Write(escaped); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// escapeFormula keeps a spreadsheet from running a cell as a formula when it
// opens an exported CSV. The quote is stripped again on import.
func escapeFormula(v string) string {
	if v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + v
	}
	return v
}

func unescapeFormula(v string) string {
	if len(v) > 1 && v[0] == '\'' && strings.ContainsRune("=+-@", rune(v[1])) {
		return v[1:]
	}
	return v
}

func writeXLSX(w io.Writer, cols []Column, texts Texts, rows [][]string) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", termsSheet); err != nil {
		return err
	}
	header, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"EDEDED"}, Pattern: 1}})
	if err != nil {
		return err
	}
	text, err := f.NewStyle(&excelize.Style{NumFmt: 49})
	if err != nil {
		return err
	}
	if _, err := f.NewSheet(valuesSheet); err != nil {
		return err
	}
	valuesColumn := 0
	for i, c := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		colName, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetCellStr(termsSheet, cell, c.ID); err != nil {
			return err
		}
		comment := texts.Label(c)
		if help := texts.Help(c); help != "" && help != comment {
			comment += "\n" + help
		}
		if c.Required {
			comment += "\n(required)"
		}
		if err := f.AddComment(termsSheet, excelize.Comment{Cell: cell, Author: "Marmot", Text: comment}); err != nil {
			return err
		}
		width := float64(max(len(c.ID), 12)) + 4
		if c.ID == ColumnDefinition || c.ID == ColumnDescription {
			width = 48
		}
		if err := f.SetColWidth(termsSheet, colName, colName, width); err != nil {
			return err
		}
		// Text cells keep dates and codes exactly as typed.
		if err := f.SetCellStyle(termsSheet, colName+"2", fmt.Sprintf("%s%d", colName, validationRows), text); err != nil {
			return err
		}
		options := choices(c)
		if len(options) == 0 || c.list() {
			continue
		}
		valuesColumn++
		valuesCol, _ := excelize.ColumnNumberToName(valuesColumn)
		if err := f.SetCellStr(valuesSheet, valuesCol+"1", c.ID); err != nil {
			return err
		}
		for j, v := range options {
			if err := f.SetCellStr(valuesSheet, fmt.Sprintf("%s%d", valuesCol, j+2), v); err != nil {
				return err
			}
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = fmt.Sprintf("%s2:%s%d", colName, colName, validationRows)
		dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$2:$%s$%d", valuesSheet, valuesCol, valuesCol, len(options)+1))
		if err := f.AddDataValidation(termsSheet, dv); err != nil {
			return err
		}
	}
	last, _ := excelize.CoordinatesToCellName(len(cols), 1)
	if err := f.SetCellStyle(termsSheet, "A1", last, header); err != nil {
		return err
	}
	if err := f.SetPanes(termsSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellStr(termsSheet, cell, v); err != nil {
				return err
			}
		}
	}
	if err := writeGuide(f, cols, texts, header); err != nil {
		return err
	}
	if valuesColumn == 0 {
		if err := f.DeleteSheet(valuesSheet); err != nil {
			return err
		}
	} else if err := f.SetSheetVisible(valuesSheet, false); err != nil {
		return err
	}
	f.SetActiveSheet(0)
	_, err = f.WriteTo(w)
	return err
}

func writeGuide(f *excelize.File, cols []Column, texts Texts, header int) error {
	if _, err := f.NewSheet(guideSheet); err != nil {
		return err
	}
	rows := [][]string{{"column", "label", "type", "required", "values", "help"}}
	for _, c := range cols {
		kind := c.Type
		if c.list() {
			kind = "list of " + c.ItemType + ", separated by " + ListSeparator
		}
		required := ""
		if c.Required {
			required = "yes"
		}
		rows = append(rows, []string{c.ID, texts.Label(c), kind, required, strings.Join(choices(c), ", "), texts.Help(c)})
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(guideSheet, cell, v); err != nil {
				return err
			}
		}
	}
	for col, width := range map[string]float64{"A": 20, "B": 28, "C": 30, "D": 10, "E": 40, "F": 70} {
		if err := f.SetColWidth(guideSheet, col, col, width); err != nil {
			return err
		}
	}
	return f.SetCellStyle(guideSheet, "A1", "F1", header)
}

// choices are the values a cell may take, for drop-downs and the guide.
func choices(c Column) []string {
	kind := c.Type
	if c.list() {
		kind = c.ItemType
	}
	switch kind {
	case "enum":
		return c.Values
	case "boolean":
		return []string{"true", "false"}
	}
	return nil
}

// Sheet is an upload read into a header and its rows. Line numbers count the
// header as line 1, as a spreadsheet shows them.
type Sheet struct {
	Header []string
	Rows   [][]string
}

// Read loads an upload within the limits. Formulas are never evaluated: an
// XLSX gives its stored values, a CSV its text.
func Read(r io.Reader, format Format) (*Sheet, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	var rows [][]string
	switch format {
	case FormatXLSX:
		rows, err = readXLSX(data)
	case FormatCSV:
		rows, err = readCSV(data)
	default:
		err = ErrUnsupportedFormat
	}
	if err != nil {
		return nil, err
	}
	for len(rows) > 0 && blank(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}
	if len(rows) == 0 || blank(rows[0]) {
		return nil, ErrNoHeader
	}
	if len(rows)-1 > MaxRows {
		return nil, ErrTooManyRows
	}
	header := make([]string, len(rows[0]))
	for i, h := range rows[0] {
		header[i] = strings.ToLower(strings.TrimSpace(h))
	}
	sheet := &Sheet{Header: header}
	for _, row := range rows[1:] {
		cells := make([]string, len(header))
		for i := range cells {
			if i < len(row) {
				cells[i] = strings.TrimSpace(unescapeFormula(row[i]))
			}
		}
		sheet.Rows = append(sheet.Rows, cells)
	}
	return sheet, nil
}

func readXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: maxUnzipped, UnzipXMLSizeLimit: maxUnzipped})
	if err != nil {
		return nil, fmt.Errorf("reading the XLSX: %w", err)
	}
	defer func() { _ = f.Close() }()
	sheet := termsSheet
	if idx, _ := f.GetSheetIndex(termsSheet); idx < 0 {
		sheet = f.GetSheetName(0)
	}
	return f.GetRows(sheet)
}

func readCSV(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sniffDelimiter(data)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading the CSV: %w", err)
	}
	return rows, nil
}

// sniffDelimiter tells ';' from ',' by the header line: spreadsheets in many
// European locales export CSV with ';'.
func sniffDelimiter(data []byte) rune {
	line, _ := bufio.NewReader(bytes.NewReader(data)).ReadString('\n')
	if strings.Count(line, ";") > strings.Count(line, ",") {
		return ';'
	}
	return ','
}

func blank(row []string) bool {
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// parseBool accepts what people type in a spreadsheet, not only true/false.
func parseBool(v string) (bool, bool) {
	switch strings.ToLower(v) {
	case "true", "yes", "y", "1", "sí", "si":
		return true, true
	case "false", "no", "n", "0":
		return false, true
	}
	b, err := strconv.ParseBool(v)
	return b, err == nil
}
