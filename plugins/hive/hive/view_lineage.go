package hive

import (
	"context"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	"github.com/rs/zerolog/log"
)

// lineageCatalog stands in for the catalog level the Trino parser expects
// above a database; Hive has none, so every source shares this one.
const lineageCatalog = "hive"

// hiveView is a view's original query, kept until every database is read.
type hiveView struct {
	database string
	name     string
	mrn      string
	query    string
}

// addLineageInputs describes a discovered table or view to the SQL parser
// and records a view's query for attachColumnLineage. The columns come from
// DESCRIBE FORMATTED, so they are there even when the run leaves them off
// the asset.
func (d *discovery) addLineageInputs(database, table string, info *tableInfo, mrnValue string) {
	cols := map[string]string{}
	for _, c := range columns(info) {
		cols[c.Name] = c.DataType
	}
	d.lineageSources = append(d.lineageSources, sqllineage.ExtractSource{
		MRN:      mrnValue,
		Database: lineageCatalog,
		Schema:   database,
		Table:    table,
		Columns:  cols,
	})
	if info.isView() && info.OriginalQuery != "" {
		d.views = append(d.views, hiveView{database: database, name: table, mrn: mrnValue, query: info.OriginalQuery})
	}
}

// attachColumnLineage parses each view's original query with the Trino
// dialect and hangs the column edges it derives on the matching VIEW_OF
// edge. There is no HiveQL parser, so only views written in the SQL the
// two share get column lineage: backtick-quoted names, LATERAL VIEW and
// other Hive-only syntax fail to parse and keep their table-level edges.
// The expanded query is not tried because Hive quotes every name in it
// with backticks. A table the parser finds but the reference scan missed
// gets an edge of its own.
func (d *discovery) attachColumnLineage(ctx context.Context) {
	if len(d.views) == 0 {
		return
	}

	edgeIdx := make(map[string]int, len(d.lineage))
	for i, e := range d.lineage {
		if e.Type == "VIEW_OF" {
			edgeIdx[e.Source+">"+e.Target] = i
		}
	}

	var extractor sqllineage.Extractor

	for _, v := range d.views {
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             v.query,
			Dialect:         "trino",
			DefaultDatabase: lineageCatalog,
			DefaultSchema:   v.database,
			Sources:         d.lineageSources,
		})
		if err != nil {
			log.Debug().Err(err).Str("view", qualifiedName(v.database, v.name)).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		for sourceMRN, colEdges := range result {
			if sourceMRN == v.mrn {
				continue
			}
			key := sourceMRN + ">" + v.mrn
			if i, ok := edgeIdx[key]; ok {
				d.lineage[i].ColumnLineage = colEdges
				continue
			}
			edgeIdx[key] = len(d.lineage)
			d.lineage = append(d.lineage, pluginsdk.LineageEdge{
				Source:        sourceMRN,
				Target:        v.mrn,
				Type:          "VIEW_OF",
				ColumnLineage: colEdges,
			})
		}
	}
}
