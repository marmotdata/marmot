package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	icetable "github.com/apache/iceberg-go/table"
	"github.com/apache/iceberg-go/view"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// viewMetadataJSON is view metadata as a REST catalog serves it, with one
// representation per (dialect, sql) pair.
func viewMetadataJSON(defaultCatalog string, reps ...[2]string) string {
	var list []map[string]string
	for _, r := range reps {
		list = append(list, map[string]string{"type": "sql", "dialect": r[0], "sql": r[1]})
	}
	repsJSON, _ := json.Marshal(list)
	catalog := ""
	if defaultCatalog != "" {
		catalog = fmt.Sprintf(`"default-catalog": %q,`, defaultCatalog)
	}
	return fmt.Sprintf(`{
  "view-uuid": "fa6506c3-7681-40c8-86dc-e36561f83385",
  "format-version": 1,
  "location": "s3://lake/warehouse/shop/customer_orders",
  "current-version-id": 1,
  "properties": {},
  "versions": [{
    "version-id": 1,
    "timestamp-ms": 1573518431292,
    "schema-id": 1,
    "summary": {"engine-name": "trino"},
    %s
    "default-namespace": ["shop"],
    "representations": %s
  }],
  "schemas": [{
    "schema-id": 1,
    "type": "struct",
    "fields": [
      {"id": 1, "name": "id", "required": false, "type": "long"},
      {"id": 2, "name": "full_name", "required": false, "type": "string"},
      {"id": 3, "name": "total", "required": false, "type": "double"}
    ]
  }],
  "version-log": [{"timestamp-ms": 1573518431292, "version-id": 1}]
}`, catalog, repsJSON)
}

func loadView(t *testing.T, ident icetable.Identifier, metadataJSON string) *view.View {
	t.Helper()
	meta, err := view.ParseMetadataString(metadataJSON)
	require.NoError(t, err)
	return view.New(ident, meta, "s3://lake/warehouse/shop/customer_orders/metadata/v1.json")
}

// tableAsset mirrors what createTableAsset emits for a table's namespace,
// name and schema.
func tableAsset(t *testing.T, s *Source, ident icetable.Identifier, cols ...string) pluginsdk.Asset {
	t.Helper()
	fields := make([]schemaField, 0, len(cols))
	for _, c := range cols {
		fields = append(fields, schemaField{Name: c, Type: "string"})
	}
	raw, err := json.Marshal(fields)
	require.NoError(t, err)
	mrnValue, _ := s.assetMRN("Table", ident)
	name := ident[len(ident)-1]
	return pluginsdk.Asset{
		Name:     &name,
		MRN:      &mrnValue,
		Type:     "Table",
		Metadata: map[string]any{"namespace": namespacePath(ident)},
		Schema:   map[string]string{"columns": string(raw)},
	}
}

func shopAssets(t *testing.T, s *Source) []pluginsdk.Asset {
	return []pluginsdk.Asset{
		tableAsset(t, s, icetable.Identifier{"shop", "customers"}, "id", "first_name", "last_name"),
		tableAsset(t, s, icetable.Identifier{"shop", "orders"}, "id", "customer_id", "total"),
	}
}

const customerOrdersSQL = `SELECT c.id, concat(c.first_name, ' ', c.last_name) AS full_name, o.total
FROM lakehouse.shop.customers c
JOIN orders o ON o.customer_id = c.id`

func discoverView(t *testing.T, s *Source, metadataJSON string) (pluginsdk.Asset, []viewDefinition) {
	t.Helper()
	ident := icetable.Identifier{"shop", "customer_orders"}
	v := loadView(t, ident, metadataJSON)
	a := s.createViewAsset(v, ident)
	var defs []viewDefinition
	if def, ok := newViewDefinition(v, ident, *a.MRN); ok {
		defs = append(defs, def)
	}
	return a, defs
}

func edgeFrom(edges []pluginsdk.LineageEdge, source string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source {
			return &edges[i]
		}
	}
	return nil
}

func TestViewLineage_TrinoViewCarriesColumnLineage(t *testing.T) {
	s := newMRNSource("")
	view, defs := discoverView(t, s, viewMetadataJSON("lakehouse", [2]string{"trino", customerOrdersSQL}))
	assets := append(shopAssets(t, s), view)

	assert.Equal(t, "trino", view.Metadata["sql_dialect"])

	edges := viewLineage(context.Background(), assets, defs)
	require.Len(t, edges, 2)

	customers := edgeFrom(edges, "mrn://asset/table/iceberg/shop.customers")
	require.NotNil(t, customers)
	assert.Equal(t, "mrn://asset/view/iceberg/shop.customer_orders", customers.Target)
	assert.Equal(t, "VIEW_OF", customers.Type)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1},
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)

	orders := edgeFrom(edges, "mrn://asset/table/iceberg/shop.orders")
	require.NotNil(t, orders)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
	}, orders.ColumnLineage)
}

func TestViewLineage_WithoutADefaultCatalogOnlyCatalogFreeNamesResolve(t *testing.T) {
	// Without a default catalog the engine's catalog name is unknown, so
	// lakehouse.shop.customers names a catalog the parser has no tables
	// for, while the unqualified orders still resolves.
	s := newMRNSource("")
	view, defs := discoverView(t, s, viewMetadataJSON("", [2]string{"trino", customerOrdersSQL}))

	edges := viewLineage(context.Background(), append(shopAssets(t, s), view), defs)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://asset/table/iceberg/shop.orders", edges[0].Source)
}

func TestViewLineage_PicksTheTrinoRepresentationOverSpark(t *testing.T) {
	s := newMRNSource("")
	view, defs := discoverView(t, s, viewMetadataJSON("lakehouse",
		[2]string{"spark", "SELECT `id` FROM `shop`.`customers`"},
		[2]string{"Presto", "SELECT id FROM shop.customers"},
	))

	assert.Equal(t, "spark", view.Metadata["sql_dialect"], "the metadata names the representation shown as the query")
	require.Len(t, defs, 1)
	assert.Equal(t, "presto", defs[0].dialect)

	edges := viewLineage(context.Background(), append(shopAssets(t, s), view), defs)
	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://asset/table/iceberg/shop.customers", edges[0].Source)
}

func TestViewLineage_SkipsSparkOnlyViews(t *testing.T) {
	s := newMRNSource("")
	_, defs := discoverView(t, s, viewMetadataJSON("", [2]string{"spark", "SELECT id FROM shop.customers"}))

	assert.Empty(t, defs)
}

func TestViewLineage_SkipsViewsTheParserRejects(t *testing.T) {
	s := newMRNSource("")
	view, defs := discoverView(t, s, viewMetadataJSON("lakehouse", [2]string{"trino", "SELECT id FROM customers WHERE ((("}))

	assert.Empty(t, viewLineage(context.Background(), append(shopAssets(t, s), view), defs))
}
