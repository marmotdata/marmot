package dbt

import (
	"context"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discoverFixture runs Discover over testdata/column_lineage: two sources,
// a seed and two models, one a staging view and one a mart joining all three.
func discoverFixture(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	var s Source
	result, err := s.Discover(context.Background(), pluginsdk.RawConfig{
		"target_path":      "testdata/column_lineage",
		"project_name":     "shop",
		"include_manifest": true,
		"include_catalog":  true,
		"discover_models":  true,
		"discover_sources": true,
	})
	require.NoError(t, err)
	return result
}

func dependsOn(edges []pluginsdk.LineageEdge, source, target string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Type == "DEPENDS_ON" && edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}

func byToColumn(e *pluginsdk.LineageEdge) map[string]pluginsdk.ColumnEdge {
	out := map[string]pluginsdk.ColumnEdge{}
	for _, ce := range e.ColumnLineage {
		out[ce.ToColumn] = ce
	}
	return out
}

const (
	rawOrdersMRN      = "mrn://asset/table/postgresql/analytics.raw.orders"
	rawCustomersMRN   = "mrn://asset/table/postgresql/analytics.raw.customers"
	countryCodesMRN   = "mrn://asset/table/postgresql/analytics.seeds.country_codes"
	stgOrdersViewMRN  = "mrn://asset/view/postgresql/analytics.staging.stg_orders"
	stgOrdersModelMRN = "mrn://asset/model/dbt/analytics.staging.stg_orders"
	customerOrdersMRN = "mrn://asset/model/dbt/analytics.marts.customer_orders"
)

func TestColumnLineage_StagingModelRenamesASourceColumn(t *testing.T) {
	result := discoverFixture(t)
	assertLineageOnlyReferencesDiscoveredAssets(t, result.Assets, result.Lineage)

	e := dependsOn(result.Lineage, rawOrdersMRN, stgOrdersModelMRN)
	require.NotNil(t, e, "expected orders→stg_orders edge; got %+v", result.Lineage)

	cols := byToColumn(e)
	assert.Equal(t, []string{"id"}, cols["order_id"].FromColumns)
	assert.Equal(t, []string{"customer_id"}, cols["customer_id"].FromColumns)
	assert.Equal(t, []string{"amount"}, cols["amount"].FromColumns)
	assert.NotContains(t, cols, "created_at")
}

// The mart reads a model, a source by its identifier and a seed whose
// columns are only in the manifest; each edge gets only its own columns.
func TestColumnLineage_MartSplitsColumnsAcrossUpstreams(t *testing.T) {
	result := discoverFixture(t)

	stg := dependsOn(result.Lineage, stgOrdersViewMRN, customerOrdersMRN)
	require.NotNil(t, stg, "expected stg_orders→customer_orders edge; got %+v", result.Lineage)
	cols := byToColumn(stg)
	assert.Equal(t, []string{"amount"}, cols["total_amount"].FromColumns)
	assert.Equal(t, "expression", cols["total_amount"].Transform)

	customers := dependsOn(result.Lineage, rawCustomersMRN, customerOrdersMRN)
	require.NotNil(t, customers, "expected customers→customer_orders edge; got %+v", result.Lineage)
	cols = byToColumn(customers)
	assert.Equal(t, []string{"id"}, cols["customer_id"].FromColumns)
	assert.Equal(t, []string{"first_name", "last_name"}, cols["customer_name"].FromColumns)
	assert.Equal(t, "expression", cols["customer_name"].Transform)

	seed := dependsOn(result.Lineage, countryCodesMRN, customerOrdersMRN)
	require.NotNil(t, seed, "expected country_codes→customer_orders edge; got %+v", result.Lineage)
	cols = byToColumn(seed)
	assert.Equal(t, []string{"country_name"}, cols["country_name"].FromColumns)
	assert.Empty(t, cols["country_name"].Transform)
}

// Adapters without a parser, and SQL the parser cannot read, keep the
// table-level edges with no column lineage.
func TestColumnLineage_FallsBackToTableLevel(t *testing.T) {
	tests := map[string]func(m *DBTManifest){
		"unsupported adapter": func(m *DBTManifest) { m.Metadata.AdapterType = "databricks" },
		"unparseable sql": func(m *DBTManifest) {
			for id, n := range m.Nodes {
				n.CompiledCode = "selec broken from"
				m.Nodes[id] = n
			}
		},
		"no compiled sql": func(m *DBTManifest) {
			for id, n := range m.Nodes {
				n.CompiledCode = ""
				m.Nodes[id] = n
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Source{config: &Config{ProjectName: "shop", TargetPath: "testdata/column_lineage", IncludeManifest: true, IncludeCatalog: true, DiscoverSources: true}}
			require.NoError(t, s.loadArtifacts(context.Background()))
			mutate(s.manifest)

			_, lineages := s.discoverModels(context.Background())

			var deps int
			for _, e := range lineages {
				if e.Type == "DEPENDS_ON" {
					deps++
					assert.Empty(t, e.ColumnLineage, "edge %s→%s", e.Source, e.Target)
				}
			}
			assert.Equal(t, 4, deps, "table-level edges are kept")
		})
	}
}

// Each mapped adapter resolves its own name forms: BigQuery's backtick
// project paths, Snowflake's upper-case catalog, and MySQL and Oracle where
// dbt's schema is the parser's database. Columns carry types because the
// Redshift parser yields nothing for columns it cannot type.
func TestColumnLineage_AdapterDialects(t *testing.T) {
	tests := []struct {
		adapter, database, schema, sql string
		upper                          bool
		// plain marks parsers that report arithmetic results as plain
		// fields, so no "expression" transform is expected.
		plain bool
	}{
		{adapter: "bigquery", database: "my-project", schema: "shop", sql: "select id, amount * 2 as doubled from `my-project`.`shop`.`orders`"},
		{adapter: "snowflake", database: "ANALYTICS", schema: "SHOP", sql: "select id, amount * 2 as doubled from analytics.shop.orders", upper: true},
		{adapter: "mysql", schema: "shop", sql: "select id, amount * 2 as doubled from `shop`.`orders`"},
		{adapter: "oracle", database: "ORCL", schema: "SHOP", sql: `select id, amount * 2 as doubled from "SHOP"."ORDERS"`, upper: true},
		{adapter: "sqlserver", database: "analytics", schema: "dbo", sql: "select id, amount * 2 as doubled from [analytics].[dbo].[orders]"},
		{adapter: "redshift", database: "dev", schema: "shop", sql: `select id, amount * 2 as doubled from "dev"."shop"."orders"`},
		{adapter: "trino", database: "hive", schema: "shop", sql: `select id, amount * 2 as doubled from "hive"."shop"."orders"`, plain: true},
	}
	for _, tt := range tests {
		t.Run(tt.adapter, func(t *testing.T) {
			name := func(s string) string {
				if tt.upper {
					return strings.ToUpper(s)
				}
				return s
			}
			cols := func(names ...string) map[string]NodeColumn {
				out := map[string]NodeColumn{}
				for _, n := range names {
					out[name(n)] = NodeColumn{Name: name(n), DataType: "integer"}
				}
				return out
			}
			s := sourceWith(&DBTManifest{
				Metadata: ManifestMetadata{AdapterType: tt.adapter},
				Nodes: map[string]ManifestNode{"model.shop.doubled": {
					UniqueID: "model.shop.doubled", Name: "doubled", ResourceType: "model",
					Database: tt.database, Schema: tt.schema, Materialized: "table",
					Columns:      cols("id", "doubled"),
					DependsOn:    NodeDependency{Nodes: []string{"source.shop.shop.orders"}},
					CompiledCode: tt.sql,
				}},
				Sources: map[string]ManifestNode{"source.shop.shop.orders": {
					UniqueID: "source.shop.shop.orders", Name: name("orders"), ResourceType: "source",
					Database: tt.database, Schema: tt.schema, Columns: cols("id", "amount"),
				}},
			}, true)

			_, lineages := s.discoverModels(context.Background())

			var e *pluginsdk.LineageEdge
			for i := range lineages {
				if lineages[i].Type == "DEPENDS_ON" {
					e = &lineages[i]
				}
			}
			require.NotNil(t, e)
			got := byToColumn(e)
			require.Contains(t, got, name("doubled"), "edges: %+v", e.ColumnLineage)
			assert.Equal(t, []string{name("amount")}, got[name("doubled")].FromColumns)
			if !tt.plain {
				assert.Equal(t, "expression", got[name("doubled")].Transform)
			}
			assert.Equal(t, []string{name("id")}, got[name("id")].FromColumns)
		})
	}
}
