package hive

import (
	"context"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// record does what discoverObject does with a described object, minus the
// HiveServer2 round trips.
func record(d *discovery, database, table string, info *tableInfo) string {
	asset := d.source.objectAsset(database, table, info, nil, false, "")
	d.assets = append(d.assets, asset)
	d.known[strings.ToLower(qualifiedName(database, table))] = *asset.MRN
	d.addLineageInputs(database, table, info, *asset.MRN)
	if info.isView() {
		for _, ref := range viewSources(info, database) {
			d.pending = append(d.pending, pendingEdge{ref: ref, mrn: *asset.MRN, refIsSource: true, edgeType: "VIEW_OF"})
		}
	}
	return *asset.MRN
}

func recordFixture(t *testing.T, d *discovery, table string) string {
	t.Helper()
	return record(d, "sales", table, parseDescribeFormatted(loadDescribe(t, table)))
}

func newDiscovery() *discovery {
	return &discovery{source: newSource(), known: make(map[string]string)}
}

func viewEdge(edges []pluginsdk.LineageEdge, source, target string) *pluginsdk.LineageEdge {
	for i := range edges {
		if edges[i].Type == "VIEW_OF" && edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}

func TestAttachColumnLineage_ViewEdgesCarryColumnLineage(t *testing.T) {
	d := newDiscovery()
	orders := recordFixture(t, d, "orders")
	customers := recordFixture(t, d, "customers")
	view := recordFixture(t, d, "daily_totals")

	d.resolvePendingEdges()
	d.attachColumnLineage(context.Background())

	require.Len(t, d.lineage, 2)

	fromOrders := viewEdge(d.lineage, orders, view)
	require.NotNil(t, fromOrders)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"amount"}, ToColumn: "total", Confidence: 1},
	}, fromOrders.ColumnLineage)

	fromCustomers := viewEdge(d.lineage, customers, view)
	require.NotNil(t, fromCustomers)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"name"}, ToColumn: "name", Confidence: 1},
	}, fromCustomers.ColumnLineage)
}

func TestAttachColumnLineage_MaterializedView(t *testing.T) {
	d := newDiscovery()
	stock := recordFixture(t, d, "stock")
	view := recordFixture(t, d, "stock_totals")

	d.resolvePendingEdges()
	d.attachColumnLineage(context.Background())

	e := viewEdge(d.lineage, stock, view)
	require.NotNil(t, e)
	assert.ElementsMatch(t, []string{"sku", "total"}, []string{e.ColumnLineage[0].ToColumn, e.ColumnLineage[1].ToColumn})
}

func TestAttachColumnLineage_HiveOnlySyntaxKeepsTheTableEdge(t *testing.T) {
	d := newDiscovery()
	stock := recordFixture(t, d, "stock")
	info := &tableInfo{
		Columns:       []columnInfo{{Name: "sku", DataType: "string"}},
		Details:       map[string]string{"Table Type": "VIRTUAL_VIEW"},
		OriginalQuery: "SELECT `sku` FROM `sales`.`stock`",
	}
	view := record(d, "sales", "quoted", info)

	d.resolvePendingEdges()
	d.attachColumnLineage(context.Background())

	require.Len(t, d.lineage, 1)
	e := viewEdge(d.lineage, stock, view)
	require.NotNil(t, e)
	assert.Empty(t, e.ColumnLineage)
}
