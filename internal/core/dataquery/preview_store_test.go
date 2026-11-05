package dataquery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// An asset is previewed only through sources that answer queries, and the table list can be narrowed to that one asset.
func TestPreviewAssetLookups(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	schedule := func(name string, queryable bool) string {
		var id string
		require.NoError(t, db.QueryRow(ctx, `INSERT INTO ingestion_schedules (name, plugin_id, config, cron_expression, enabled, queryable) VALUES ($1, 'duckdb', '{}', '', true, $2) RETURNING id`,
			name, queryable).Scan(&id))
		return id
	}
	asset := func(id, name, schema string) {
		_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, metadata, created_by) VALUES ($1, $2, $3, 'Table', '{DuckDB}', jsonb_build_object('schema', $4::text, 'table_name', $5::text), 'test')`,
			id, name, "mrn://table/duckdb/"+schema+"."+name, schema, name)
		require.NoError(t, err)
	}
	link := func(assetID, scheduleID string) {
		_, err := db.Exec(ctx, `INSERT INTO asset_schedules (asset_id, schedule_id) VALUES ($1, $2)`, assetID, scheduleID)
		require.NoError(t, err)
	}

	served := schedule("local-duckdb", true)
	catalogOnly := schedule("nightly-duckdb", false)
	asset("a-orders", "orders", "sales")
	asset("a-customers", "customers", "sales")
	link("a-orders", served)
	link("a-customers", served)
	link("a-orders", catalogOnly)

	names, err := repo.QueryableSourcesForAsset(ctx, "a-orders")
	require.NoError(t, err)
	assert.Equal(t, []string{"local-duckdb"}, names, "a pipeline that does not answer queries is never offered")

	names, err = repo.QueryableSourcesForAsset(ctx, "a-unlinked")
	require.NoError(t, err)
	assert.Empty(t, names)

	tables, meta, err := repo.ListSourceTables(ctx, SourceTablesFilter{SourceID: served, AssetID: "a-orders", Limit: 10})
	require.NoError(t, err)
	require.Len(t, tables, 1)
	assert.Equal(t, "orders", tables[0].Name)
	assert.Equal(t, "SELECT *\nFROM sales.orders\nLIMIT 10", sourceTableLocators["duckdb"].statement(meta[tables[0].MRN], tables[0].Name))
}
