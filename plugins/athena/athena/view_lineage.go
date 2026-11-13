package athena

import (
	"context"
	"maps"
	"slices"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	"github.com/rs/zerolog/log"
)

// viewDefinition is one view's decoded SQL and where it lives, kept until
// every catalog is read because a view may read tables of another
// database or catalog.
type viewDefinition struct {
	catalog  string
	database string
	name     string
	mrn      string
	sql      string
}

// addSource describes a discovered table or view to the SQL parser. The
// columns come from the catalog's own listing, so they are there even
// when the run leaves them off the asset.
func (i *tableIndex) addSource(catalog, database string, t catalogTable, mrnValue string) {
	cols := make(map[string]string, len(t.Columns)+len(t.PartitionKeys))
	for _, c := range t.Columns {
		cols[c.Name] = c.Type
	}
	for _, c := range t.PartitionKeys {
		cols[c.Name] = c.Type
	}
	i.sources = append(i.sources, sqllineage.ExtractSource{
		MRN:      mrnValue,
		Database: catalog,
		Schema:   database,
		Table:    t.Name,
		Columns:  cols,
	})
}

func (i *tableIndex) addView(v viewDefinition) {
	i.views = append(i.views, v)
}

// viewEdges builds a VIEW_OF edge from each discovered table a view reads
// and attaches the column lineage the SQL parser derives from the view.
// The FROM/JOIN scan finds the tables, so a view the parser cannot handle
// keeps its table-level edges; a table only the parser finds gets an edge
// of its own.
func (i *tableIndex) viewEdges(ctx context.Context) []pluginsdk.LineageEdge {
	var extractor sqllineage.Extractor
	var edges []pluginsdk.LineageEdge

	for _, v := range i.views {
		edgeIdx := map[string]int{}
		for _, ref := range extractQueryTables(v.sql) {
			source, ok := i.resolve(ref, v.database)
			if !ok || source == v.mrn {
				continue
			}
			if _, duplicate := edgeIdx[source]; duplicate {
				continue
			}
			edgeIdx[source] = len(edges)
			edges = append(edges, edge(source, v.mrn, "VIEW_OF"))
		}

		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             v.sql,
			Dialect:         "athena",
			DefaultDatabase: v.catalog,
			DefaultSchema:   v.database,
			Sources:         i.sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", v.database+"."+v.name).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		for _, sourceMRN := range slices.Sorted(maps.Keys(result)) {
			if sourceMRN == v.mrn {
				continue
			}
			if idx, ok := edgeIdx[sourceMRN]; ok {
				edges[idx].ColumnLineage = result[sourceMRN]
				continue
			}
			e := edge(sourceMRN, v.mrn, "VIEW_OF")
			e.ColumnLineage = result[sourceMRN]
			edges = append(edges, e)
		}
	}

	return edges
}
