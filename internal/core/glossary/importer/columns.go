// Package importer turns glossary spreadsheets into terms and back: it builds
// a template from the metamodel profile, reads XLSX or CSV uploads, validates
// every row and exports the glossary in the same shape.
package importer

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const (
	ColumnName        = "name"
	ColumnDefinition  = "definition"
	ColumnDescription = "description"
	ColumnParent      = "parent"
	ColumnOwners      = "owners"
	ColumnTags        = "tags"
)

// ListSeparator splits list cells. A comma would clash with CSV and with the
// prose in definitions.
const ListSeparator = "|"

// Column is one column of the template. Headers are the stable IDs: they are
// what an upload is read by, whatever language the labels are in.
type Column struct {
	ID       string
	Type     string
	ItemType string
	Required bool
	Values   []string
	// Validation holds a profile field's constraints: range, length, items.
	Validation metamodel.Constraints
	// Format names how a term's own column is written ("name", "text",
	// "term", "owners", "tags"); profile columns are described by their type.
	Format string
	// Storage is the metadata.* binding of a profile field; empty for the
	// term's own columns.
	Storage  string
	LabelKey string
	HelpKey  string
	// Label and Help are the English text used when the profile's messages
	// have none for LabelKey and HelpKey.
	Label string
	Help  string
}

func (c Column) profile() bool { return c.Storage != "" }

func (c Column) list() bool { return c.Type == "list" }

// Columns lists the term's own columns followed by the profile fields that
// apply to glossary_term and are stored in metadata.
func Columns(registry *metamodel.Registry) []Column {
	cols := []Column{
		{ID: ColumnName, Type: "string", Required: true, LabelKey: "glossary.import.name", Label: "Name", Help: "The term's name. It identifies the term: rows with a name that already exists update or skip that term.", Format: "name"},
		{ID: ColumnDefinition, Type: "string", Required: true, LabelKey: "glossary.import.definition", Label: "Definition", Help: "What the term means. Required for new terms.", Format: "text"},
		{ID: ColumnDescription, Type: "string", LabelKey: "glossary.import.description", Label: "Description", Help: "Optional longer description.", Format: "text"},
		{ID: ColumnParent, Type: "string", LabelKey: "glossary.import.parent", Label: "Parent term", Help: "Name of the parent term, existing or in this file.", Format: "term"},
		{ID: ColumnOwners, Type: "list", ItemType: "string", Required: true, LabelKey: "glossary.import.owners", Label: "Owners", Help: "Usernames, or team:<team name>, separated by |. At least one for new terms.", Format: "owners"},
		{ID: ColumnTags, Type: "list", ItemType: "string", LabelKey: "glossary.import.tags", Label: "Tags", Help: "Tags separated by |.", Format: "tags"},
	}
	if registry == nil || !registry.Enabled() {
		return cols
	}
	for _, f := range registry.Fields("glossary_term") {
		if !strings.HasPrefix(f.Storage, "metadata.") {
			continue
		}
		cols = append(cols, Column{
			ID:         f.ID,
			Type:       f.Type,
			ItemType:   f.ItemType,
			Required:   f.Required,
			Values:     f.Values,
			Validation: f.Validation,
			Storage:    f.Storage,
			LabelKey:   f.Presentation.LabelKey,
			HelpKey:    f.Presentation.HelpTextKey,
			Label:      f.ID,
		})
	}
	return cols
}

// Texts resolves column labels and help from the profile's messages: the
// requested locale, then the profile's default locale, then the English
// fallback. The server has no other catalogue.
type Texts struct {
	messages      map[string]map[string]string
	locale        string
	defaultLocale string
}

func NewTexts(schema metamodel.Schema, locale string) Texts {
	return Texts{messages: schema.Messages, locale: locale, defaultLocale: schema.DefaultLocale}
}

func (t Texts) resolve(key, fallback string) string {
	if key == "" {
		return fallback
	}
	for _, locale := range []string{t.locale, t.defaultLocale} {
		if text, ok := t.messages[locale][key]; ok && text != "" {
			return text
		}
	}
	return fallback
}

func (t Texts) Label(c Column) string { return t.resolve(c.LabelKey, c.Label) }

func (t Texts) Help(c Column) string { return t.resolve(c.HelpKey, c.Help) }

// Describe explains in English how to write a column's cells, for the XLSX
// guide sheet; the web UI builds its own localized text from the same data.
func Describe(c Column) string {
	switch c.Format {
	case "name":
		return "Text. Identifies the term ignoring case: a row matches an existing term that differs only in case, and keeps its name."
	case "term":
		return "The name of another term, existing or in this file, ignoring case."
	case "owners":
		return "Usernames, or team:<team name>, separated by " + ListSeparator + ", ignoring case."
	case "tags":
		return "Tags separated by " + ListSeparator + ". Case is kept: PII and pii are different tags."
	case "text":
		return "Text."
	}
	kind := c.Type
	if c.list() {
		kind = c.ItemType
	}
	var text string
	switch kind {
	case "string":
		text = "Text"
	case "integer":
		text = "Whole number"
	case "number":
		text = "Number (a dot or comma for decimals)"
	case "boolean":
		text = "true or false (also yes/no), ignoring case"
	case "date":
		text = "Date as YYYY-MM-DD"
	case "enum":
		text = "One of the allowed values, ignoring case; stored as the profile spells it"
	default:
		text = kind
	}
	v := c.Validation
	if v.Minimum != nil && v.Maximum != nil {
		text += fmt.Sprintf(", from %g to %g", *v.Minimum, *v.Maximum)
	} else if v.Minimum != nil {
		text += fmt.Sprintf(", at least %g", *v.Minimum)
	} else if v.Maximum != nil {
		text += fmt.Sprintf(", at most %g", *v.Maximum)
	}
	if v.MinLength != nil {
		text += fmt.Sprintf(", at least %d characters", *v.MinLength)
	}
	if v.MaxLength != nil {
		text += fmt.Sprintf(", at most %d characters", *v.MaxLength)
	}
	if c.list() {
		text = "Several values separated by " + ListSeparator + ", each: " + strings.ToLower(text[:1]) + text[1:]
		if v.MinItems != nil {
			text += fmt.Sprintf("; at least %d", *v.MinItems)
		}
		if v.MaxItems != nil {
			text += fmt.Sprintf("; at most %d", *v.MaxItems)
		}
	}
	return text + "."
}

// ColumnInfo is a column as clients show it in their own guide.
type ColumnInfo struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Help       string                 `json:"help,omitempty"`
	Type       string                 `json:"type"`
	ItemType   string                 `json:"item_type,omitempty"`
	Required   bool                   `json:"required"`
	Values     []string               `json:"values,omitempty"`
	Validation *metamodel.Constraints `json:"validation,omitempty"`
	Format     string                 `json:"format,omitempty"`
	Separator  string                 `json:"separator,omitempty"`
	Profile    bool                   `json:"profile"`
}

func (c Column) info(texts Texts) ColumnInfo {
	out := ColumnInfo{ID: c.ID, Label: texts.Label(c), Help: texts.Help(c), Type: c.Type, ItemType: c.ItemType, Required: c.Required, Values: choices(c), Format: c.Format, Profile: c.profile()}
	if c.list() {
		out.Separator = ListSeparator
	}
	if v := c.Validation; v.Minimum != nil || v.Maximum != nil || v.MinLength != nil || v.MaxLength != nil || v.MinItems != nil || v.MaxItems != nil {
		out.Validation = &v
	}
	return out
}
