package iceberg

import (
	"cmp"
	"context"
	"encoding/json"
	"strings"

	icetable "github.com/apache/iceberg-go/table"
	"github.com/apache/iceberg-go/view"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	"github.com/rs/zerolog/log"
)

// lineageDialects are the view SQL dialects the parser reads. A view only
// written in another dialect, such as spark, gets no lineage.
var lineageDialects = map[string]bool{"trino": true, "presto": true, "athena": true}

// viewDefinition is the SQL representation of a view the parser can read,
// with the catalog and namespace its unqualified names resolve against.
type viewDefinition struct {
	mrn              string
	name             string
	sql              string
	dialect          string
	defaultCatalog   string
	defaultNamespace string
}

// newViewDefinition picks the first representation of the view's current
// version in a dialect the parser reads.
func newViewDefinition(v *view.View, ident icetable.Identifier, mrnValue string) (viewDefinition, bool) {
	ver := v.CurrentVersion()
	if ver == nil {
		return viewDefinition{}, false
	}
	for _, rep := range ver.Representations {
		dialect := strings.ToLower(rep.Dialect)
		if !lineageDialects[dialect] || rep.Sql == "" {
			continue
		}
		return viewDefinition{
			mrn:              mrnValue,
			name:             strings.Join(ident, "."),
			sql:              rep.Sql,
			dialect:          dialect,
			defaultCatalog:   ver.DefaultCatalog,
			defaultNamespace: cmp.Or(strings.Join(ver.DefaultNamespace, "."), namespacePath(ident)),
		}, true
	}
	return viewDefinition{}, false
}

// viewLineage emits a VIEW_OF edge, carrying column lineage, from every
// discovered table or view a view's SQL reads. A view the parser cannot
// handle gets no edges.
func viewLineage(ctx context.Context, assets []pluginsdk.Asset, defs []viewDefinition) []pluginsdk.LineageEdge {
	if len(defs) == 0 {
		return nil
	}

	sources := buildExtractSources(assets)
	if len(sources) == 0 {
		return nil
	}

	var extractor sqllineage.Extractor
	var edges []pluginsdk.LineageEdge

	for _, def := range defs {
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             def.sql,
			Dialect:         def.dialect,
			DefaultDatabase: def.defaultCatalog,
			DefaultSchema:   def.defaultNamespace,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", def.name).Msg("view lineage extract failed; skipping view lineage")
			continue
		}
		for sourceMRN, colEdges := range result {
			if sourceMRN == def.mrn {
				continue
			}
			edges = append(edges, pluginsdk.LineageEdge{
				Source:        sourceMRN,
				Target:        def.mrn,
				Type:          "VIEW_OF",
				ColumnLineage: colEdges,
			})
		}
	}

	return edges
}

// buildExtractSources describes every discovered table and view to the
// parser. Iceberg does not know the engine's catalog name, so Database is
// left to default to each view's default catalog; a view without one only
// resolves names that leave the catalog out.
func buildExtractSources(assets []pluginsdk.Asset) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assets))
	for _, a := range assets {
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil || a.Name == nil {
			continue
		}
		namespace, _ := a.Metadata["namespace"].(string)
		sources = append(sources, sqllineage.ExtractSource{
			MRN:     *a.MRN,
			Schema:  namespace,
			Table:   *a.Name,
			Columns: decodeColumns(a.Schema["columns"]),
		})
	}
	return sources
}

// decodeColumns pulls (name → type) out of the JSON extractSchemaFields
// stores in Asset.Schema["columns"].
func decodeColumns(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var fields []schemaField
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return out
	}
	for _, f := range fields {
		if f.Name != "" {
			out[f.Name] = f.Type
		}
	}
	return out
}
