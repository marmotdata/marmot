package importer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/metamodel"
)

// ErrTooManyTerms refuses an export that could not be imported back.
var ErrTooManyTerms = fmt.Errorf("the glossary has more than %d terms, the most one file can import", MaxRows)

// Apply writes the rows of a valid result, all or none. It validates nothing
// itself: call it only when Valid reports true.
func (im *Importer) Apply(ctx context.Context, svc glossary.Service, result *Result) error {
	if !result.Valid() {
		return fmt.Errorf("%w: the file has errors", glossary.ErrInvalidInput)
	}
	terms := result.Terms()
	if len(terms) == 0 {
		return nil
	}
	if _, err := svc.Import(ctx, terms); err != nil {
		return err
	}
	result.Applied = true
	return nil
}

// Export reads the whole glossary as rows of the template's columns, so the
// file can be edited and imported back with OnExistingUpdate.
func (im *Importer) Export(ctx context.Context, svc glossary.Service) ([][]string, error) {
	const page = 500
	var terms []*glossary.GlossaryTerm
	for offset := 0; ; offset += page {
		list, err := svc.List(ctx, offset, page)
		if err != nil {
			return nil, err
		}
		terms = append(terms, list.Terms...)
		if len(terms) > MaxRows {
			return nil, ErrTooManyTerms
		}
		if len(list.Terms) < page {
			break
		}
	}
	names := make(map[string]string, len(terms))
	for _, t := range terms {
		names[t.ID] = t.Name
	}
	cols := im.Columns()
	rows := make([][]string, 0, len(terms))
	for _, listed := range terms {
		// List leaves owners out; Get has them.
		t, err := svc.Get(ctx, listed.ID)
		if err != nil {
			return nil, err
		}
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = exportCell(c, t, names)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func exportCell(c Column, t *glossary.GlossaryTerm, names map[string]string) string {
	switch c.ID {
	case ColumnName:
		return t.Name
	case ColumnDefinition:
		return t.Definition
	case ColumnDescription:
		if t.Description != nil {
			return *t.Description
		}
		return ""
	case ColumnParent:
		if t.ParentTermID != nil {
			return names[*t.ParentTermID]
		}
		return ""
	case ColumnOwners:
		refs := make([]string, 0, len(t.Owners))
		for _, o := range t.Owners {
			if o.Type == "team" {
				refs = append(refs, "team:"+o.Name)
			} else if o.Username != nil {
				refs = append(refs, *o.Username)
			}
		}
		return strings.Join(refs, ListSeparator)
	case ColumnTags:
		return strings.Join(t.Tags, ListSeparator)
	}
	if !c.profile() {
		return ""
	}
	value, ok := metamodel.ValueAt(t.Metadata, c.Storage)
	if !ok || value == nil {
		return ""
	}
	if items, ok := value.([]any); ok {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, scalarText(item))
		}
		return strings.Join(parts, ListSeparator)
	}
	return scalarText(value)
}

func scalarText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}
