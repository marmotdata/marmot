package importer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/metamodel"
)

type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionSkip   Action = "skip"
	ActionError  Action = "error"
)

// OnExisting says what to do with a row whose name matches a term.
type OnExisting string

const (
	OnExistingSkip   OnExisting = "skip"
	OnExistingUpdate OnExisting = "update"
)

// Problem is one issue with one cell, or with the row when Column is empty.
type Problem struct {
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Row struct {
	Line     int       `json:"line"`
	Name     string    `json:"name"`
	Action   Action    `json:"action"`
	Errors   []Problem `json:"errors,omitempty"`
	Warnings []Problem `json:"warnings,omitempty"`
	term     glossary.ImportTerm
}

type Summary struct {
	Create int `json:"create"`
	Update int `json:"update"`
	Skip   int `json:"skip"`
	Errors int `json:"errors"`
}

type Result struct {
	Rows []Row `json:"rows"`
	// Problems concern the file as a whole, such as a missing column, and
	// block applying it; Warnings, such as an unknown column, do not.
	Problems []Problem `json:"problems,omitempty"`
	Warnings []Problem `json:"warnings,omitempty"`
	Summary  Summary   `json:"summary"`
	Applied  bool      `json:"applied"`
}

// Valid reports whether the file may be applied: no file or row errors.
func (r *Result) Valid() bool { return len(r.Problems) == 0 && r.Summary.Errors == 0 }

// Terms looks terms up by name. Names are not unique in Marmot, so it returns
// every live term with each name.
type Terms interface {
	ByNames(ctx context.Context, names []string) (map[string][]*glossary.GlossaryTerm, error)
}

// Owners resolves an owner cell entry: a username, or team:<name>.
type Owners interface {
	ResolveOwner(ctx context.Context, ref string) (glossary.OwnerInput, error)
}

// ErrOwnerNotFound is what an Owners returns for an unknown user or team.
var ErrOwnerNotFound = errors.New("owner not found")

type Importer struct {
	registry *metamodel.Registry
	terms    Terms
	owners   Owners
}

func New(registry *metamodel.Registry, terms Terms, owners Owners) *Importer {
	return &Importer{registry: registry, terms: terms, owners: owners}
}

func (im *Importer) Columns() []Column { return Columns(im.registry) }

// Describe lists the template's columns for a client's guide, in locale.
func (im *Importer) Describe(locale string) []ColumnInfo {
	texts := im.Texts(locale)
	cols := im.Columns()
	out := make([]ColumnInfo, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.info(texts))
	}
	return out
}

// Texts resolves labels in locale from the profile's messages.
func (im *Importer) Texts(locale string) Texts {
	if im.registry == nil {
		return Texts{locale: locale}
	}
	return NewTexts(im.registry.SchemaForKind("glossary_term"), locale)
}

// Validate checks every row without writing anything.
func (im *Importer) Validate(ctx context.Context, sheet *Sheet, onExisting OnExisting) (*Result, error) {
	cols := im.Columns()
	result := &Result{Rows: []Row{}}
	index := map[string]int{}
	for i, h := range sheet.Header {
		if h == "" {
			continue
		}
		if _, dup := index[h]; dup {
			result.Problems = append(result.Problems, Problem{Column: h, Code: "duplicate_column", Message: fmt.Sprintf("column %q appears twice", h)})
			continue
		}
		index[h] = i
	}
	known := map[string]Column{}
	for _, c := range cols {
		known[c.ID] = c
	}
	for _, h := range sheet.Header {
		if _, ok := known[h]; h != "" && !ok {
			result.Warnings = append(result.Warnings, Problem{Column: h, Code: "unknown_column", Message: fmt.Sprintf("column %q is not in the template and is ignored", h)})
		}
	}
	if _, ok := index[ColumnName]; !ok {
		result.Problems = append(result.Problems, Problem{Column: ColumnName, Code: "missing_column", Message: "the name column is required"})
		return result, nil
	}
	cell := func(row []string, id string) string {
		if i, ok := index[id]; ok {
			return row[i]
		}
		return ""
	}

	names := make([]string, 0, len(sheet.Rows))
	lines := map[string][]int{}
	for i, row := range sheet.Rows {
		if blank(row) {
			continue
		}
		name := cell(row, ColumnName)
		names = append(names, name)
		lines[name] = append(lines[name], i+2)
	}
	existing, err := im.terms.ByNames(ctx, names)
	if err != nil {
		return nil, err
	}

	for i, row := range sheet.Rows {
		if blank(row) {
			continue
		}
		r := Row{Line: i + 2, Name: cell(row, ColumnName)}
		im.validateRow(ctx, &r, row, cell, known, index, existing, lines, onExisting)
		result.Rows = append(result.Rows, r)
	}
	im.checkParents(result.Rows)

	for i := range result.Rows {
		r := &result.Rows[i]
		if len(r.Errors) > 0 {
			r.Action = ActionError
		}
		switch r.Action {
		case ActionCreate:
			result.Summary.Create++
		case ActionUpdate:
			result.Summary.Update++
		case ActionSkip:
			result.Summary.Skip++
		case ActionError:
			result.Summary.Errors++
		}
	}
	return result, nil
}

func (im *Importer) validateRow(ctx context.Context, r *Row, row []string, cell func([]string, string) string, known map[string]Column, index map[string]int, existing map[string][]*glossary.GlossaryTerm, lines map[string][]int, onExisting OnExisting) {
	fail := func(column, code, message string) {
		r.Errors = append(r.Errors, Problem{Column: column, Code: code, Message: message})
	}
	if r.Name == "" {
		fail(ColumnName, "required", "a name is required")
		return
	}
	if len(lines[r.Name]) > 1 {
		fail(ColumnName, "duplicate_in_file", fmt.Sprintf("the name is repeated on lines %v", lines[r.Name]))
	}
	matches := existing[r.Name]
	var current *glossary.GlossaryTerm
	switch {
	case len(matches) > 1:
		fail(ColumnName, "ambiguous_name", fmt.Sprintf("%d terms already have this name; rename them before importing", len(matches)))
		return
	case len(matches) == 1:
		current = matches[0]
		if onExisting != OnExistingUpdate {
			r.Action = ActionSkip
			return
		}
		r.Action = ActionUpdate
	default:
		r.Action = ActionCreate
	}

	definition, description := cell(row, ColumnDefinition), cell(row, ColumnDescription)
	if current == nil && definition == "" {
		fail(ColumnDefinition, "required", "a definition is required for a new term")
	}
	parent := cell(row, ColumnParent)
	if parent == r.Name {
		fail(ColumnParent, "parent_self", "a term cannot be its own parent")
	} else if parent != "" && len(existing[parent]) == 0 && len(lines[parent]) == 0 {
		fail(ColumnParent, "parent_not_found", fmt.Sprintf("no term named %q exists or is in the file", parent))
	} else if len(existing[parent]) > 1 {
		fail(ColumnParent, "ambiguous_parent", fmt.Sprintf("%d terms are named %q", len(existing[parent]), parent))
	}
	owners := im.readOwners(ctx, cell(row, ColumnOwners), fail)
	if current == nil && len(owners) == 0 && !slices.ContainsFunc(r.Errors, func(p Problem) bool { return p.Column == ColumnOwners }) {
		fail(ColumnOwners, "required", "at least one owner is required for a new term")
	}
	tags := splitList(cell(row, ColumnTags))

	var metadata map[string]interface{}
	if current != nil {
		metadata = deepCopy(current.Metadata)
	}
	values := map[string]any{}
	for id, i := range index {
		c, ok := known[id]
		if !ok || !c.profile() || row[i] == "" {
			continue
		}
		value, ok := parseValue(c, row[i])
		if !ok {
			fail(c.ID, "type", fmt.Sprintf("%q is not a valid %s", row[i], describe(c)))
			continue
		}
		values[c.ID] = value
		metadata = setPath(metadata, c.Storage, value)
	}
	if im.registry != nil && im.registry.Enabled() {
		all := map[string]any{}
		for _, c := range known {
			if c.profile() {
				if v, ok := metamodel.ValueAt(metadata, c.Storage); ok {
					all[c.ID] = v
				}
			}
		}
		var validation *metamodel.ValidationError
		if err := im.registry.Validate(all, "glossary_term", true); errors.As(err, &validation) {
			for _, v := range validation.Fields {
				if _, parsedHere := values[v.Field]; parsedHere || current == nil {
					fail(v.Field, v.Code, fmt.Sprintf("%s does not satisfy the profile (%s)", v.Field, v.Code))
				}
			}
		}
		for _, v := range im.registry.Missing(all, "glossary_term", true) {
			r.Warnings = append(r.Warnings, Problem{Column: v.Field, Code: "missing", Message: fmt.Sprintf("%s is required by the profile and has no value", v.Field)})
		}
	}

	if current == nil {
		in := glossary.CreateTermInput{Name: r.Name, Definition: definition, Owners: owners, Tags: tags, Metadata: metadata}
		if description != "" {
			in.Description = &description
		}
		r.term = glossary.ImportTerm{Create: in, ParentName: parent}
		return
	}
	// On update, an empty cell keeps the current value.
	in := glossary.UpdateTermInput{Metadata: metadata}
	if definition != "" {
		in.Definition = &definition
	}
	if description != "" {
		in.Description = &description
	}
	if len(owners) > 0 {
		in.Owners = owners
	}
	if len(tags) > 0 {
		in.Tags = tags
	}
	r.term = glossary.ImportTerm{ExistingID: current.ID, Update: in, ParentName: parent}
}

func (im *Importer) readOwners(ctx context.Context, value string, fail func(column, code, message string)) []glossary.OwnerInput {
	var owners []glossary.OwnerInput
	for _, ref := range splitList(value) {
		owner, err := im.owners.ResolveOwner(ctx, ref)
		if err != nil {
			code := "owner_error"
			if errors.Is(err, ErrOwnerNotFound) {
				code = "owner_not_found"
			}
			fail(ColumnOwners, code, fmt.Sprintf("owner %q: %v", ref, err))
			continue
		}
		owners = append(owners, owner)
	}
	return owners
}

// checkParents marks rows whose parents, within the file, loop back to them.
func (im *Importer) checkParents(rows []Row) {
	parentOf := map[string]string{}
	for _, r := range rows {
		if r.Action == ActionCreate || r.Action == ActionUpdate {
			parentOf[r.Name] = r.term.ParentName
		}
	}
	for i := range rows {
		seen := map[string]bool{rows[i].Name: true}
		for p := parentOf[rows[i].Name]; p != ""; p = parentOf[p] {
			if seen[p] {
				rows[i].Errors = append(rows[i].Errors, Problem{Column: ColumnParent, Code: "parent_cycle", Message: "the parent terms in the file form a loop"})
				break
			}
			seen[p] = true
		}
	}
}

// Terms returns what to write for the rows that create or update a term.
func (r *Result) Terms() []glossary.ImportTerm {
	var terms []glossary.ImportTerm
	for _, row := range r.Rows {
		if row.Action == ActionCreate || row.Action == ActionUpdate {
			terms = append(terms, row.term)
		}
	}
	return terms
}

func splitList(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ListSeparator) {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func describe(c Column) string {
	if c.list() {
		return "list of " + c.ItemType
	}
	return c.Type
}

func parseValue(c Column, raw string) (any, bool) {
	if !c.list() {
		return parseScalar(c.Type, c.Values, raw)
	}
	items := splitList(raw)
	out := make([]any, 0, len(items))
	for _, item := range items {
		v, ok := parseScalar(c.ItemType, c.Values, item)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

func parseScalar(kind string, values []string, raw string) (any, bool) {
	switch kind {
	case "string":
		return raw, true
	case "enum":
		return raw, slices.Contains(values, raw)
	case "boolean":
		return parseBool(raw)
	case "integer":
		n, err := strconv.ParseInt(raw, 10, 64)
		return float64(n), err == nil
	case "number":
		n, err := strconv.ParseFloat(strings.Replace(raw, ",", ".", 1), 64)
		return n, err == nil
	case "date":
		if _, err := time.Parse("2006-01-02", raw); err == nil {
			return raw, true
		}
		// A date cell a spreadsheet stored as a serial number.
		if serial, err := strconv.ParseFloat(raw, 64); err == nil && serial > 0 {
			return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial)).Format("2006-01-02"), true
		}
		return nil, false
	}
	return nil, false
}

func setPath(m map[string]interface{}, storage string, value any) map[string]interface{} {
	path := strings.Split(strings.TrimPrefix(storage, "metadata."), ".")
	if m == nil {
		m = map[string]interface{}{}
	}
	cursor := m
	for _, part := range path[:len(path)-1] {
		next, ok := cursor[part].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cursor[part] = next
		}
		cursor = next
	}
	cursor[path[len(path)-1]] = value
	return m
}

func deepCopy(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	maps.Copy(out, m)
	for k, v := range out {
		if child, ok := v.(map[string]interface{}); ok {
			out[k] = deepCopy(child)
		}
	}
	return out
}
