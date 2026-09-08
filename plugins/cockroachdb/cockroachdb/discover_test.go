package cockroachdb

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualifiedName_JoinsDatabaseSchemaAndTable(t *testing.T) {
	assert.Equal(t, "shop.public.orders", qualifiedName("shop", "public", "orders"))
}

func TestTableIdent_QuotesEveryPart(t *testing.T) {
	assert.Equal(t, `"shop"."public"."orders"`, tableIdent("shop", "public", "orders"))
}

func TestTableIdent_EscapesEmbeddedQuotes(t *testing.T) {
	// A name with a quote in it must not be able to break out of the
	// identifier and inject SQL into the sample data query.
	assert.Equal(t, `"shop"."public"."say ""hi"""`, tableIdent("shop", "public", `say "hi"`))
}

func TestBuildColumns_DropsHiddenColumns(t *testing.T) {
	// rowid on a table without a primary key and the shard column of a
	// hash-sharded index are CockroachDB's own bookkeeping.
	rows := []columnRow{
		{Schema: "public", Table: "audit_log", Name: "message", DataType: "STRING", Nullable: true},
		{Schema: "public", Table: "audit_log", Name: "rowid", DataType: "INT8", Hidden: true},
		{Schema: "public", Table: "events", Name: "crdb_internal_id_shard_8", DataType: "INT8", Hidden: true},
		{Schema: "public", Table: "events", Name: "id", DataType: "INT8"},
	}

	columns := buildColumns(rows, nil)

	require.Len(t, columns["public.audit_log"], 1)
	assert.Equal(t, "message", columns["public.audit_log"][0].Name)
	require.Len(t, columns["public.events"], 1)
	assert.Equal(t, "id", columns["public.events"][0].Name)
}

func TestBuildColumns_MarksPrimaryKeys(t *testing.T) {
	rows := []columnRow{
		{Schema: "public", Table: "customers", Name: "id", DataType: "INT8"},
		{Schema: "public", Table: "customers", Name: "name", DataType: "STRING"},
	}
	primaryKeys := map[string]struct{}{"public.customers.id": {}}

	columns := buildColumns(rows, primaryKeys)

	require.Len(t, columns["public.customers"], 2)
	assert.True(t, columns["public.customers"][0].PrimaryKey)
	assert.False(t, columns["public.customers"][1].PrimaryKey)
}

func TestBuildColumns_PrimaryKeyIsScopedToItsTable(t *testing.T) {
	// The same column name in another table must not inherit the key.
	rows := []columnRow{
		{Schema: "public", Table: "orders", Name: "id", DataType: "INT8"},
		{Schema: "sales", Table: "orders", Name: "id", DataType: "INT8"},
	}
	primaryKeys := map[string]struct{}{"public.orders.id": {}}

	columns := buildColumns(rows, primaryKeys)

	assert.True(t, columns["public.orders"][0].PrimaryKey)
	assert.False(t, columns["sales.orders"][0].PrimaryKey)
}

func TestBuildColumns_KeepsTheGenerationExpression(t *testing.T) {
	rows := []columnRow{
		{Schema: "public", Table: "orders", Name: "total_with_tax", DataType: "DECIMAL", Nullable: true, Generation: "total * 1.21"},
	}

	columns := buildColumns(rows, nil)

	require.Len(t, columns["public.orders"], 1)
	assert.Equal(t, "total * 1.21", columns["public.orders"][0].GenerationExpression)
}

func TestBuildColumns_SetsTheDefaultOnlyWhenPresent(t *testing.T) {
	status := "'new'"
	rows := []columnRow{
		{Schema: "public", Table: "orders", Name: "status", DataType: "STRING", Nullable: true, Default: &status},
		{Schema: "public", Table: "orders", Name: "total", DataType: "DECIMAL(10,2)"},
	}

	columns := buildColumns(rows, nil)

	require.Len(t, columns["public.orders"], 2)
	assert.Equal(t, "'new'", columns["public.orders"][0].Default)
	assert.Nil(t, columns["public.orders"][1].Default)
}

func TestBuildColumns_CarriesTheColumnComment(t *testing.T) {
	rows := []columnRow{
		{Schema: "public", Table: "customers", Name: "email", DataType: "STRING", Nullable: true, Comment: "Primary contact address"},
	}

	columns := buildColumns(rows, nil)

	assert.Equal(t, "Primary contact address", columns["public.customers"][0].Description)
}

func TestColumn_SerialisesWithTheCanonicalKeys(t *testing.T) {
	status := "'new'"
	columns := buildColumns([]columnRow{
		{Schema: "public", Table: "orders", Name: "status", DataType: "STRING", Nullable: true, Default: &status},
		{Schema: "public", Table: "orders", Name: "total_with_tax", DataType: "DECIMAL", Nullable: true, Generation: "total * 1.21"},
	}, nil)

	data, err := json.Marshal(columns["public.orders"])
	require.NoError(t, err)

	var decoded []map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded, 2)

	assert.Equal(t, "status", decoded[0]["column_name"])
	assert.Equal(t, "STRING", decoded[0]["data_type"])
	assert.Equal(t, true, decoded[0]["is_nullable"])
	assert.Equal(t, "'new'", decoded[0]["default_expression"])
	assert.NotContains(t, decoded[0], "generation_expression")
	assert.Equal(t, "total * 1.21", decoded[1]["generation_expression"])
	assert.NotContains(t, decoded[1], "default_expression")
}

func TestSelectDatabases_DropsTheExcludedOnes(t *testing.T) {
	all := []database{{Name: "analytics"}, {Name: "defaultdb"}, {Name: "shop"}, {Name: "system"}}

	selected, err := selectDatabases(all, "", []string{"system", "defaultdb"})
	require.NoError(t, err)

	assert.Equal(t, []database{{Name: "analytics"}, {Name: "shop"}}, selected)
}

func TestSelectDatabases_ExplicitDatabaseWinsOverExclusions(t *testing.T) {
	all := []database{{Name: "shop"}, {Name: "system"}}

	selected, err := selectDatabases(all, "system", []string{"system"})
	require.NoError(t, err)

	assert.Equal(t, []database{{Name: "system"}}, selected)
}

func TestSelectDatabases_ExplicitDatabaseMustExist(t *testing.T) {
	all := []database{{Name: "shop"}}

	_, err := selectDatabases(all, "missing", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestWithoutViews_KeepsTablesOnly(t *testing.T) {
	relations := []relation{
		{Schema: "public", Name: "orders", Kind: relkindTable},
		{Schema: "public", Name: "customer_totals", Kind: relkindView},
		{Schema: "public", Name: "order_summary", Kind: relkindMaterializedView},
	}

	kept := withoutViews(relations)

	require.Len(t, kept, 1)
	assert.Equal(t, "orders", kept[0].Name)
}

func TestSplitColumnNames_TrimsAndDropsEmptyParts(t *testing.T) {
	assert.Equal(t, []string{"region", "id"}, splitColumnNames("region, id"))
	assert.Nil(t, splitColumnNames(""))
}

func TestRelation_ViewsAreViewsAndTheRestAreTables(t *testing.T) {
	assert.Equal(t, "View", relation{Kind: relkindView}.assetType())
	assert.Equal(t, "View", relation{Kind: relkindMaterializedView}.assetType())
	assert.Equal(t, "Table", relation{Kind: relkindTable}.assetType())
	assert.Equal(t, "Table", relation{Kind: relkindPartitionedTable}.assetType())
	assert.Equal(t, "Table", relation{Kind: relkindForeignTable}.assetType())
}

func TestRelation_ObjectTypeNamesTheKind(t *testing.T) {
	assert.Equal(t, "table", relation{Kind: relkindTable}.objectType())
	assert.Equal(t, "view", relation{Kind: relkindView}.objectType())
	assert.Equal(t, "materialized_view", relation{Kind: relkindMaterializedView}.objectType())
	assert.Equal(t, "foreign_table", relation{Kind: relkindForeignTable}.objectType())
}

func newSource() *Source {
	return &Source{config: &Config{Host: "localhost", Port: 26257, User: "root"}}
}

func TestRelationAsset_NameIsFullyQualified(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	require.NotNil(t, asset.Name)
	assert.Equal(t, "shop.public.orders", *asset.Name)
	assert.Equal(t, "Table", asset.Type)
	assert.Equal(t, []string{"CockroachDB"}, asset.Providers)
	assert.Equal(t, "shop", asset.Metadata["database"])
	assert.Equal(t, "public", asset.Metadata["schema"])
	assert.Equal(t, "orders", asset.Metadata["table_name"])
	assert.Equal(t, "table", asset.Metadata["object_type"])
}

func TestRelationAsset_CommentBecomesTheDescription(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "customers", Kind: relkindTable, Comment: "People who buy things"}, relationDetails{})

	require.NotNil(t, asset.Description)
	assert.Equal(t, "People who buy things", *asset.Description)
	assert.Equal(t, "People who buy things", asset.Metadata["comment"])
}

func TestRelationAsset_NoCommentMeansNoDescription(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	assert.Nil(t, asset.Description)
	assert.NotContains(t, asset.Metadata, "comment")
}

func TestRelationAsset_ViewCarriesItsDefinitionAsTheQuery(t *testing.T) {
	definition := "SELECT status, count(*) AS n FROM shop.public.orders GROUP BY status"
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "order_summary", Kind: relkindMaterializedView}, relationDetails{definition: definition})

	assert.Equal(t, "View", asset.Type)
	require.NotNil(t, asset.Query)
	assert.Equal(t, definition, *asset.Query)
	require.NotNil(t, asset.QueryLanguage)
	assert.Equal(t, "SQL", *asset.QueryLanguage)
}

func TestRelationAsset_MaterializedViewIsFlagged(t *testing.T) {
	materialized := newSource().relationAsset("shop", relation{Schema: "public", Name: "order_summary", Kind: relkindMaterializedView}, relationDetails{})
	plain := newSource().relationAsset("shop", relation{Schema: "public", Name: "customer_totals", Kind: relkindView}, relationDetails{})

	assert.Equal(t, true, materialized.Metadata["materialized"])
	assert.Equal(t, "materialized_view", materialized.Metadata["object_type"])
	assert.NotContains(t, plain.Metadata, "materialized")
	assert.Equal(t, "view", plain.Metadata["object_type"])
}

func TestRelationAsset_PartitionedTableListsItsColumns(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "shipments", Kind: relkindTable}, relationDetails{
		partition: &partition{Columns: []string{"region", "id"}},
	})

	assert.Equal(t, true, asset.Metadata["partitioned"])
	assert.Equal(t, "region, id", asset.Metadata["partition_columns"])
}

func TestRelationAsset_UnpartitionedTableHasNoPartitionKeys(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	assert.NotContains(t, asset.Metadata, "partitioned")
	assert.NotContains(t, asset.Metadata, "partition_columns")
}

func TestRelationAsset_RowCountLandsInMetadata(t *testing.T) {
	count := int64(3)
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{rowCount: &count})

	assert.Equal(t, int64(3), asset.Metadata["estimated_row_count"])
}

func TestRelationAsset_ColumnsAreSerialisedIntoTheSchema(t *testing.T) {
	columns := []column{{Column: pluginsdk.Column{Name: "id", DataType: "INT8", PrimaryKey: true}}}
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{columns: columns})

	raw, ok := asset.Schema["columns"]
	require.True(t, ok)

	var decoded []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, "id", decoded[0]["column_name"])
	assert.Equal(t, true, decoded[0]["is_primary_key"])
}

func TestRelationAsset_NoColumnsWhenColumnDiscoveryIsOff(t *testing.T) {
	asset := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	assert.NotContains(t, asset.Schema, "columns")
}

func TestRelationAsset_InterpolatesTags(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 26257, User: "root", BaseConfig: pluginsdk.BaseConfig{Tags: []string{"db:${database}", "static"}}}}

	asset := s.relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	assert.Equal(t, []string{"db:shop", "static"}, asset.Tags)
}

func TestDatabaseAsset_CarriesConnectionAndVersion(t *testing.T) {
	asset := newSource().databaseAsset(database{Name: "shop", Owner: "root"}, "CockroachDB CCL v25.2.23")

	require.NotNil(t, asset.Name)
	assert.Equal(t, "shop", *asset.Name)
	assert.Equal(t, "Database", asset.Type)
	assert.Equal(t, "localhost", asset.Metadata["host"])
	assert.Equal(t, 26257, asset.Metadata["port"])
	assert.Equal(t, "shop", asset.Metadata["database"])
	assert.Equal(t, "root", asset.Metadata["owner"])
	assert.Equal(t, "CockroachDB CCL v25.2.23", asset.Metadata["server_version"])
}

func TestViewLineage_LinksAViewToEveryBaseRelation(t *testing.T) {
	relations := []relation{
		{Schema: "public", Name: "orders", Kind: relkindTable},
		{Schema: "public", Name: "customers", Kind: relkindTable},
		{Schema: "public", Name: "customer_totals", Kind: relkindView},
	}
	known := map[string]relation{}
	for _, r := range relations {
		known[r.key()] = r
	}
	definitions := map[string]string{
		"public.customer_totals": "SELECT c.id, sum(o.total) FROM shop.public.orders AS o JOIN shop.public.customers AS c ON c.id = o.customer_id GROUP BY c.id",
	}

	edges := viewLineage("shop", relations, definitions, known)

	// Edges run base table -> view, the direction the data flows.
	assert.ElementsMatch(t, []pluginsdk.LineageEdge{
		{Source: "mrn://table/cockroachdb/shop.public.orders", Target: "mrn://view/cockroachdb/shop.public.customer_totals", Type: "VIEW_OF"},
		{Source: "mrn://table/cockroachdb/shop.public.customers", Target: "mrn://view/cockroachdb/shop.public.customer_totals", Type: "VIEW_OF"},
	}, edges)
}

func TestViewLineage_ViewOnAViewSourcesTheViewType(t *testing.T) {
	relations := []relation{
		{Schema: "public", Name: "orders", Kind: relkindTable},
		{Schema: "public", Name: "paid_orders", Kind: relkindView},
		{Schema: "public", Name: "paid_totals", Kind: relkindView},
	}
	known := map[string]relation{}
	for _, r := range relations {
		known[r.key()] = r
	}
	definitions := map[string]string{
		"public.paid_orders": "SELECT * FROM shop.public.orders WHERE status = 'paid'",
		"public.paid_totals": "SELECT sum(total) FROM shop.public.paid_orders",
	}

	edges := viewLineage("shop", relations, definitions, known)

	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://view/cockroachdb/shop.public.paid_orders",
		Target: "mrn://view/cockroachdb/shop.public.paid_totals",
		Type:   "VIEW_OF",
	})
}

func TestViewLineage_SkipsReferencesThatWereNotDiscovered(t *testing.T) {
	relations := []relation{{Schema: "public", Name: "v", Kind: relkindView}}
	known := map[string]relation{"public.v": relations[0]}
	definitions := map[string]string{"public.v": "SELECT 1 FROM other.public.t"}

	assert.Empty(t, viewLineage("shop", relations, definitions, known))
}

func TestViewLineage_SkipsViewsWithoutADefinition(t *testing.T) {
	relations := []relation{{Schema: "public", Name: "v", Kind: relkindView}}
	known := map[string]relation{"public.v": relations[0]}

	assert.Empty(t, viewLineage("shop", relations, nil, known))
}

func TestConvertValue_NilStaysNil(t *testing.T) {
	assert.Nil(t, convertValue(nil))
}

func TestConvertValue_DecimalRendersAsText(t *testing.T) {
	// pgx hands DECIMAL back as pgtype.Numeric, which would otherwise reach
	// the UI as a struct dump.
	value := pgtype.Numeric{Int: big.NewInt(10000), Exp: -2, Valid: true}

	assert.Equal(t, "100.00", convertValue(value))
}

func TestConvertValue_TimeRendersAsRFC3339(t *testing.T) {
	value := time.Date(2026, 9, 7, 12, 30, 0, 0, time.UTC)

	assert.Equal(t, "2026-09-07T12:30:00Z", convertValue(value))
}

func TestConvertValue_UUIDRendersInCanonicalForm(t *testing.T) {
	value := [16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}

	assert.Equal(t, "12345678-9abc-def0-1234-56789abcdef0", convertValue(value))
}

func TestConvertValue_BytesRenderAsHex(t *testing.T) {
	assert.Equal(t, `\x01ff`, convertValue([]byte{0x01, 0xff}))
}

func TestConvertValue_ArraysConvertEachElement(t *testing.T) {
	value := []interface{}{time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), int64(1)}

	assert.Equal(t, []interface{}{"2026-09-07T00:00:00Z", int64(1)}, convertValue(value))
}

func TestConvertValue_ScalarsPassThrough(t *testing.T) {
	assert.Equal(t, int64(42), convertValue(int64(42)))
	assert.Equal(t, "paid", convertValue("paid"))
	assert.Equal(t, true, convertValue(true))
}
