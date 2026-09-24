// Package importer turns glossary spreadsheets into terms and back: it builds
// a template from the metamodel profile, reads XLSX or CSV uploads, validates
// every row and exports the glossary in the same shape.
package importer

import (
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
		{ID: ColumnName, Type: "string", Required: true, LabelKey: "glossary.import.name", Label: "Name", Help: "The term's name. It identifies the term: rows with a name that already exists update or skip that term."},
		{ID: ColumnDefinition, Type: "string", Required: true, LabelKey: "glossary.import.definition", Label: "Definition", Help: "What the term means. Required for new terms."},
		{ID: ColumnDescription, Type: "string", LabelKey: "glossary.import.description", Label: "Description", Help: "Optional longer description."},
		{ID: ColumnParent, Type: "string", LabelKey: "glossary.import.parent", Label: "Parent term", Help: "Name of the parent term, existing or in this file."},
		{ID: ColumnOwners, Type: "list", ItemType: "string", Required: true, LabelKey: "glossary.import.owners", Label: "Owners", Help: "Usernames, or team:<team name>, separated by |. At least one for new terms."},
		{ID: ColumnTags, Type: "list", ItemType: "string", LabelKey: "glossary.import.tags", Label: "Tags", Help: "Tags separated by |."},
	}
	if registry == nil || !registry.Enabled() {
		return cols
	}
	for _, f := range registry.Fields("glossary_term") {
		if !strings.HasPrefix(f.Storage, "metadata.") {
			continue
		}
		cols = append(cols, Column{
			ID:       f.ID,
			Type:     f.Type,
			ItemType: f.ItemType,
			Required: f.Required,
			Values:   f.Values,
			Storage:  f.Storage,
			LabelKey: f.Presentation.LabelKey,
			HelpKey:  f.Presentation.HelpTextKey,
			Label:    f.ID,
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
