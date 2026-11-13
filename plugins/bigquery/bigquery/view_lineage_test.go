package bigquery

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/bigquery"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableAsset builds the asset discoverTables emits for a table or view, with
// the schema stored the same way so the lineage path reads production shape.
func tableAsset(t *testing.T, s *Source, assetType, dataset, table string, fields bigquery.Schema, extra map[string]any) pluginsdk.Asset {
	t.Helper()
	mrnValue := s.tableMRN(assetType, dataset, table)
	schemaBytes, err := json.Marshal(s.generateJSONSchema(fields, table))
	require.NoError(t, err)
	metadata := map[string]any{"project_id": s.config.ProjectID, "dataset_id": dataset, "table_id": table}
	for k, v := range extra {
		metadata[k] = v
	}
	return pluginsdk.Asset{
		Name:     &table,
		MRN:      &mrnValue,
		Type:     assetType,
		Metadata: metadata,
		Schema:   map[string]string{"json_schema": string(schemaBytes)},
	}
}

func viewLineageFixture(t *testing.T, s *Source) []pluginsdk.Asset {
	users := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "first_name", Type: bigquery.StringFieldType},
		{Name: "last_name", Type: bigquery.StringFieldType},
	}
	orders := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "user_id", Type: bigquery.IntegerFieldType},
		{Name: "total", Type: bigquery.NumericFieldType},
	}
	return []pluginsdk.Asset{
		tableAsset(t, s, "Table", "raw", "users", users, nil),
		tableAsset(t, s, "Table", "sales", "orders", orders, nil),
		// A view in a third dataset reading both, with BigQuery's fully
		// qualified backtick form and an unqualified-dataset reference.
		tableAsset(t, s, "View", "marts", "customer_spend", bigquery.Schema{
			{Name: "id", Type: bigquery.IntegerFieldType},
			{Name: "full_name", Type: bigquery.StringFieldType},
			{Name: "total_spend", Type: bigquery.NumericFieldType},
		}, map[string]any{"view_query": "SELECT u.id, CONCAT(u.first_name, ' ', u.last_name) AS full_name, SUM(o.total) AS total_spend " +
			"FROM `proj.raw.users` u JOIN `proj`.`sales`.`orders` o ON o.user_id = u.id GROUP BY 1, 2"}),
		// A materialized view is discovered as a Table with its query alongside.
		tableAsset(t, s, "Table", "sales", "daily_totals", bigquery.Schema{
			{Name: "user_id", Type: bigquery.IntegerFieldType},
			{Name: "total", Type: bigquery.NumericFieldType},
		}, map[string]any{"materialized_view_query": "SELECT user_id, SUM(total) AS total FROM orders GROUP BY user_id"}),
	}
}

func edgeBetween(edges []pluginsdk.LineageEdge, source, target string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}

func columnEdges(e *pluginsdk.LineageEdge) map[string]pluginsdk.ColumnEdge {
	out := map[string]pluginsdk.ColumnEdge{}
	for _, ce := range e.ColumnLineage {
		out[ce.ToColumn] = ce
	}
	return out
}

func TestViewLineage_ViewAcrossDatasets(t *testing.T) {
	s := newMRNSource("")
	edges := s.discoverViewLineage(context.Background(), viewLineageFixture(t, s))

	view := s.tableMRN("View", "marts", "customer_spend")

	users := edgeBetween(edges, s.tableMRN("Table", "raw", "users"), view)
	require.NotNil(t, users, "expected users→customer_spend edge; got %+v", edges)
	assert.Equal(t, "DEPENDS_ON", users.Type)
	byTarget := columnEdges(users)
	assert.Equal(t, []string{"id"}, byTarget["id"].FromColumns)
	assert.Empty(t, byTarget["id"].Transform)
	assert.Equal(t, []string{"first_name", "last_name"}, byTarget["full_name"].FromColumns)
	assert.Equal(t, "expression", byTarget["full_name"].Transform)

	orders := edgeBetween(edges, s.tableMRN("Table", "sales", "orders"), view)
	require.NotNil(t, orders, "expected orders→customer_spend edge; got %+v", edges)
	byTarget = columnEdges(orders)
	assert.Equal(t, []string{"total"}, byTarget["total_spend"].FromColumns)
	assert.Equal(t, "expression", byTarget["total_spend"].Transform)
}

// An unqualified table in a materialized view resolves against its own
// dataset.
func TestViewLineage_MaterializedView(t *testing.T) {
	s := newMRNSource("")
	edges := s.discoverViewLineage(context.Background(), viewLineageFixture(t, s))

	mv := edgeBetween(edges, s.tableMRN("Table", "sales", "orders"), s.tableMRN("Table", "sales", "daily_totals"))
	require.NotNil(t, mv, "expected orders→daily_totals edge; got %+v", edges)
	byTarget := columnEdges(mv)
	assert.Equal(t, []string{"user_id"}, byTarget["user_id"].FromColumns)
	assert.Equal(t, []string{"total"}, byTarget["total"].FromColumns)
}

// Views the parser cannot read produce no edge rather than failing discovery.
func TestViewLineage_UnparseableAndLegacySQLAreSkipped(t *testing.T) {
	s := newMRNSource("")
	assets := viewLineageFixture(t, s)[:2]
	assets = append(assets,
		tableAsset(t, s, "View", "marts", "broken", nil, map[string]any{"view_query": "SELEC nonsense FROM"}),
		tableAsset(t, s, "View", "marts", "legacy", nil, map[string]any{
			"view_query":     "SELECT id FROM [proj:raw.users]",
			"use_legacy_sql": true,
		}),
	)

	assert.Empty(t, s.discoverViewLineage(context.Background(), assets))
}
