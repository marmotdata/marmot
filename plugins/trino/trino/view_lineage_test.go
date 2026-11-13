package trino

import (
	"context"
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableWithColumns builds a table asset whose columns are stored the way
// attachColumns stores them.
func tableWithColumns(t *testing.T, s *Source, catalog, schema, table, connector string, cols ...string) pluginsdk.Asset {
	t.Helper()
	a := s.createTableAsset(catalog, schema, table, "BASE TABLE", connector)
	var rows []map[string]any
	for i, name := range cols {
		rows = append(rows, map[string]any{"column_name": name, "data_type": "varchar", "is_nullable": "YES", "ordinal_position": i + 1})
	}
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	a.Schema = map[string]string{"columns": string(raw)}
	return a
}

func edgeFrom(edges []pluginsdk.LineageEdge, source string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source {
			return &edges[i]
		}
	}
	return nil
}

func TestViewLineage_EmitsColumnLineageAcrossCatalogs(t *testing.T) {
	s := &Source{config: mappedConfig()}
	view := s.createTableAsset("lake", "sales", "customer_orders", "VIEW", "hive")
	assets := []pluginsdk.Asset{
		tableWithColumns(t, s, "pg", "public", "customers", "postgresql", "id", "first_name", "last_name"),
		tableWithColumns(t, s, "lake", "sales", "orders", "hive", "id", "customer_id", "total"),
		view,
	}
	viewSQL := map[string]string{*view.MRN: `SELECT c.id, concat(c.first_name, ' ', c.last_name) AS full_name, o.total
FROM pg.public.customers c
JOIN orders o ON o.customer_id = c.id`}

	edges := viewLineage(context.Background(), assets, viewSQL)
	require.Len(t, edges, 2)

	customers := edgeFrom(edges, "mrn://asset/table/postgresql@prod-eu/shop.public.customers")
	require.NotNil(t, customers)
	assert.Equal(t, *view.MRN, customers.Target)
	assert.Equal(t, "VIEW_OF", customers.Type)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1},
		{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)

	orders := edgeFrom(edges, "mrn://asset/table/trino/lake.sales.orders")
	require.NotNil(t, orders)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
	}, orders.ColumnLineage)
}

func TestViewLineage_ExpandsSelectStar(t *testing.T) {
	s := &Source{config: mappedConfig()}
	view := s.createTableAsset("lake", "sales", "v", "VIEW", "hive")
	assets := []pluginsdk.Asset{
		tableWithColumns(t, s, "lake", "sales", "orders", "hive", "id", "total"),
		view,
	}

	edges := viewLineage(context.Background(), assets, map[string]string{*view.MRN: "SELECT * FROM orders WHERE total > 100"})

	require.Len(t, edges, 1)
	assert.Len(t, edges[0].ColumnLineage, 2)
}

func TestViewLineage_SkipsViewsTheParserRejects(t *testing.T) {
	s := &Source{config: mappedConfig()}
	view := s.createTableAsset("lake", "sales", "v", "VIEW", "hive")
	assets := []pluginsdk.Asset{
		tableWithColumns(t, s, "lake", "sales", "orders", "hive", "id"),
		view,
	}

	assert.Empty(t, viewLineage(context.Background(), assets, map[string]string{*view.MRN: "SELECT id FROM orders WHERE ((("}))
}

func TestViewLineage_IgnoresUndiscoveredTablesAndViewsWithoutSQL(t *testing.T) {
	s := &Source{config: mappedConfig()}
	view := s.createTableAsset("lake", "sales", "v", "VIEW", "hive")
	other := s.createTableAsset("lake", "sales", "no_sql", "VIEW", "hive")
	assets := []pluginsdk.Asset{view, other}

	assert.Empty(t, viewLineage(context.Background(), assets, map[string]string{*view.MRN: "SELECT id FROM hive.raw.events"}))
}

func TestDecodeColumns_ReadsTheAttachColumnsShape(t *testing.T) {
	raw := `[{"column_name":"id","data_type":"bigint","is_nullable":"NO","ordinal_position":1}]`
	assert.Equal(t, map[string]string{"id": "bigint"}, decodeColumns(raw))
	assert.Empty(t, decodeColumns("not json"))
}
