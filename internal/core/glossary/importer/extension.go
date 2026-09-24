package importer

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/glossary"
)

// RowContext is what a ColumnProvider sees of one row: the term's name, the
// term it updates (nil for a new one) and the row's cells in the provider's
// columns.
type RowContext struct {
	Name    string
	Current *glossary.GlossaryTerm
	Cells   map[string]string
}

// ColumnProvider adds columns kept outside the term itself, such as a
// distribution's own classification of terms. Its cells travel to the
// glossary service in ImportTerm.Extra, where a decorated service applies
// them; the importer only describes, checks and exports them.
type ColumnProvider interface {
	Columns() []Column
	// Check validates a row's cells before anything is written.
	Check(ctx context.Context, row RowContext) (errs, warnings []Problem)
	// Value is a term's cell for column, for exports.
	Value(ctx context.Context, term *glossary.GlossaryTerm, column string) (string, error)
}
