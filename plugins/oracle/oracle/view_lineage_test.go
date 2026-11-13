package oracle

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	customersMRN = "mrn://asset/table/oracle/freepdb1.sales.customers"
	ordersMRN    = "mrn://asset/table/oracle/freepdb1.sales.orders"
	spendMRN     = "mrn://asset/view/oracle/freepdb1.sales.customer_spend"
)

// newLineageDiscovery is a discovery that has read the SALES schema: two
// tables and the CUSTOMER_SPEND view, with the upper-case names and the
// columns the catalog reports for unquoted identifiers.
func newLineageDiscovery(viewColumns ...string) *discovery {
	d := newTestDiscovery()
	add := func(name, mrn string, columns ...string) {
		ref := objectRef{Owner: "SALES", Name: name}
		d.objects[ref] = mrn
		rows := make([]columnRow, 0, len(columns))
		for _, c := range columns {
			rows = append(rows, columnRow{Table: name, Name: c, DataType: "NUMBER"})
		}
		d.addColumnTypes(ref, rows)
	}
	add("CUSTOMERS", customersMRN, "CUSTOMER_ID", "FIRST_NAME", "LAST_NAME")
	add("ORDERS", ordersMRN, "ORDER_ID", "CUSTOMER_ID", "TOTAL")
	add("CUSTOMER_SPEND", spendMRN, viewColumns...)
	return d
}

func viewEdgeFrom(t *testing.T, d *discovery, source string) pluginsdk.LineageEdge {
	t.Helper()
	for _, e := range d.lineage {
		if e.Source == source && e.Target == spendMRN {
			return e
		}
	}
	require.Failf(t, "edge not found", "no edge from %s in %v", source, d.lineage)
	return pluginsdk.LineageEdge{}
}

func TestAddViewLineage_CarriesColumnLineage(t *testing.T) {
	// ALL_VIEWS.TEXT is the bare query, with the case the author typed.
	d := newLineageDiscovery("CUSTOMER_NAME", "TOTAL_SPEND")
	d.texts = append(d.texts, viewText{
		ref: objectRef{Owner: "SALES", Name: "CUSTOMER_SPEND"},
		text: `SELECT c.first_name || ' ' || c.last_name AS customer_name,
       SUM(o.total) AS total_spend
FROM customers c
JOIN sales.orders o ON o.customer_id = c.customer_id
GROUP BY c.first_name, c.last_name`,
	})

	d.addViewLineage(context.Background())

	require.Len(t, d.lineage, 2, "column lineage rides on the existing VIEW_OF edges rather than adding new ones")

	customers := viewEdgeFrom(t, d, customersMRN)
	assert.Equal(t, "VIEW_OF", customers.Type)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"FIRST_NAME", "LAST_NAME"}, ToColumn: "CUSTOMER_NAME", Transform: "expression", Confidence: 1},
	}, customers.ColumnLineage)

	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"TOTAL"}, ToColumn: "TOTAL_SPEND", Transform: "expression", Confidence: 1},
	}, viewEdgeFrom(t, d, ordersMRN).ColumnLineage)
}

func TestAddViewLineage_ParsesDDLFromDBMSMetadata(t *testing.T) {
	// When TEXT cannot be read the plugin falls back to GET_DDL, which
	// returns the full CREATE statement with quoted names.
	d := newLineageDiscovery("ORDER_ID", "TOTAL")
	d.texts = append(d.texts, viewText{
		ref: objectRef{Owner: "SALES", Name: "CUSTOMER_SPEND"},
		text: `CREATE OR REPLACE FORCE NONEDITIONABLE VIEW "SALES"."CUSTOMER_SPEND" ("ORDER_ID", "TOTAL") DEFAULT COLLATION "USING_NLS_COMP"  AS
  SELECT "ORDER_ID","TOTAL" FROM "SALES"."ORDERS"
  WITH READ ONLY`,
	})

	d.addViewLineage(context.Background())

	require.Len(t, d.lineage, 1)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"ORDER_ID"}, ToColumn: "ORDER_ID", Confidence: 1},
		{FromColumns: []string{"TOTAL"}, ToColumn: "TOTAL", Confidence: 1},
	}, viewEdgeFrom(t, d, ordersMRN).ColumnLineage)
}

func TestAddViewLineage_DropsEdgesToColumnsTheViewDoesNotHave(t *testing.T) {
	// A view declared with a column list renames its query's output, and
	// TEXT holds only the query, so its names are not the view's columns.
	d := newLineageDiscovery("ID", "AMOUNT")
	d.texts = append(d.texts, viewText{
		ref:  objectRef{Owner: "SALES", Name: "CUSTOMER_SPEND"},
		text: "SELECT order_id, total FROM orders",
	})

	d.addViewLineage(context.Background())

	require.Len(t, d.lineage, 1, "the table-level edge stays")
	assert.Empty(t, d.lineage[0].ColumnLineage)
}

func TestAddViewLineage_KeepsTableEdgesWhenTheSQLDoesNotParse(t *testing.T) {
	d := newLineageDiscovery("TOTAL")
	d.texts = append(d.texts, viewText{
		ref:  objectRef{Owner: "SALES", Name: "CUSTOMER_SPEND"},
		text: "SELECT FROM orders WHERE (",
	})

	d.addViewLineage(context.Background())

	require.Len(t, d.lineage, 1)
	assert.Equal(t, ordersMRN, d.lineage[0].Source)
	assert.Empty(t, d.lineage[0].ColumnLineage)
}

func TestAddViewLineage_StarExpandsWithoutViewColumns(t *testing.T) {
	// With include_columns off the view's own columns are unknown, so the
	// edges are kept as the parser reports them.
	d := newLineageDiscovery()
	d.texts = append(d.texts, viewText{
		ref:  objectRef{Owner: "SALES", Name: "CUSTOMER_SPEND"},
		text: "SELECT * FROM orders WITH CHECK OPTION",
	})

	d.addViewLineage(context.Background())

	require.Len(t, d.lineage, 1)
	assert.ElementsMatch(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"CUSTOMER_ID"}, ToColumn: "CUSTOMER_ID", Confidence: 1},
		{FromColumns: []string{"ORDER_ID"}, ToColumn: "ORDER_ID", Confidence: 1},
		{FromColumns: []string{"TOTAL"}, ToColumn: "TOTAL", Confidence: 1},
	}, d.lineage[0].ColumnLineage)
}

func TestViewQuery(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"bare query", "SELECT id FROM t", "SELECT id FROM t"},
		{"ddl header", `CREATE OR REPLACE FORCE EDITIONABLE VIEW "HR"."V" ("A", "B") AS SELECT x, y FROM t`, "SELECT x, y FROM t"},
		{"plain create", "create view v as\nselect id from t;", "select id from t"},
		{"read only", "SELECT id FROM t WITH READ ONLY", "SELECT id FROM t"},
		{"named check option", "SELECT id FROM t WITH CHECK OPTION CONSTRAINT v_ck", "SELECT id FROM t"},
		{"with clause is kept", "WITH a AS (SELECT id FROM t) SELECT id FROM a", "WITH a AS (SELECT id FROM t) SELECT id FROM a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, viewQuery(tt.text))
		})
	}
}
