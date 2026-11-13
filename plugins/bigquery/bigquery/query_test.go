package bigquery

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

func TestClassifyStatementAllowsSelect(t *testing.T) {
	err := classifyStatement(&bigquery.QueryStatistics{StatementType: "SELECT"})
	require.NoError(t, err)
}

func TestClassifyStatementRefusesNonSelect(t *testing.T) {
	for _, statementType := range []string{"INSERT", "UPDATE", "DELETE", "MERGE", "CREATE_TABLE_AS_SELECT", "DROP_TABLE", "SCRIPT", "CALL", "EXPORT_DATA", "LOAD_DATA", ""} {
		err := classifyStatement(&bigquery.QueryStatistics{StatementType: statementType})
		require.ErrorIs(t, err, errReadOnly, statementType)
	}
}

func TestClassifyStatementRefusesADestinationEvenIfReportedAsSelect(t *testing.T) {
	// Defence in depth: a dry run that somehow reports SELECT while still
	// naming a DDL target or an export is refused too.
	err := classifyStatement(&bigquery.QueryStatistics{StatementType: "SELECT", DDLTargetTable: &bigquery.Table{TableID: "x"}})
	require.ErrorIs(t, err, errReadOnly)

	err = classifyStatement(&bigquery.QueryStatistics{StatementType: "SELECT", DDLOperationPerformed: "CREATE"})
	require.ErrorIs(t, err, errReadOnly)

	err = classifyStatement(&bigquery.QueryStatistics{StatementType: "SELECT", ExportDataStatistics: &bigquery.ExportDataStatistics{FileCount: 1}})
	require.ErrorIs(t, err, errReadOnly)
}

func TestCallsExternalQuery(t *testing.T) {
	for statement, want := range map[string]bool{
		`SELECT * FROM EXTERNAL_QUERY('connection', 'select 1 from payroll')`:                true,
		`select o.id, e.* from orders o, external_query('conn', 'select 1') e`:               true,
		"SELECT * FROM orders /* hidden */ CROSS JOIN TABLE(\n EXTERNAL_QUERY('conn', 'x'))": true,
		`SELECT * FROM orders WHERE note = 'EXTERNAL_QUERY(x)'`:                              false,
		"SELECT * FROM orders -- EXTERNAL_QUERY(x)\nWHERE id = 1":                            false,
		"SELECT * FROM orders # EXTERNAL_QUERY(x)\nWHERE id = 1":                             false,
		`SELECT """EXTERNAL_QUERY(x)"""`:                                                     false,
		`SELECT * FROM orders`:                                                               false,
		`SELECT * FROM orders WHERE note = 'unterminated`:                                    true,
		`SELECT * FROM orders /* unterminated`:                                               true,
		`SELECT """unterminated`:                                                             true,
		`SELECT * FROM orders WHERE note = 'it\'s external_query(x) escaped'`:                false,
	} {
		assert.Equal(t, want, callsExternalQuery(statement), statement)
	}
}

func TestClassifyMetadata(t *testing.T) {
	assert.Equal(t, "View", classifyMetadata(&bigquery.TableMetadata{ViewQuery: "SELECT 1"}))
	assert.Equal(t, "ExternalTable", classifyMetadata(&bigquery.TableMetadata{ExternalDataConfig: &bigquery.ExternalDataConfig{}}))
	assert.Equal(t, "Table", classifyMetadata(&bigquery.TableMetadata{}))
}

func TestReferencedMRNs(t *testing.T) {
	tables := []*bigquery.Table{
		{ProjectID: "proj", DatasetID: "staging", TableID: "orders"},
		{ProjectID: "proj", DatasetID: "prod", TableID: "orders"}, // same table id, different dataset
		{ProjectID: "proj", DatasetID: "prod", TableID: "customers"},
	}
	classify := func(_ context.Context, t *bigquery.Table) string {
		if t.TableID == "customers" {
			return "View"
		}
		return "Table"
	}

	mrns := referencedMRNs(context.Background(), &Config{}, tables, classify)

	assert.Equal(t, []string{
		"mrn://asset/table/bigquery/proj.staging.orders",
		"mrn://asset/table/bigquery/proj.prod.orders",
		"mrn://asset/view/bigquery/proj.prod.customers",
	}, mrns)
}

func TestReferencedMRNsWithNoTables(t *testing.T) {
	mrns := referencedMRNs(context.Background(), &Config{}, nil, classifyTable)
	assert.Empty(t, mrns, "the host then falls back to a whole-source grant")
}

func TestSanitizeLabelValue(t *testing.T) {
	assert.Equal(t, "alice-example-com", sanitizeLabelValue("alice@example.com"))
	assert.Equal(t, "already-valid", sanitizeLabelValue("already-valid"))
	assert.Equal(t, "", sanitizeLabelValue(""))
	assert.Equal(t, "", sanitizeLabelValue("@@@"), "an identity that sanitises to nothing comes back empty")

	assert.Equal(t, strings.Repeat("a", 63), sanitizeLabelValue(strings.Repeat("a", 100)))
}

func TestQueryLabels(t *testing.T) {
	labels := queryLabels("alice@example.com")
	assert.Equal(t, "data-query", labels["marmot"])
	assert.Equal(t, "alice-example-com", labels["marmot-identity"])

	labels = queryLabels("")
	assert.Equal(t, "data-query", labels["marmot"])
	_, ok := labels["marmot-identity"]
	assert.False(t, ok, "an identity that sanitises to nothing leaves the label off rather than send an invalid one")
}

func TestColumnType(t *testing.T) {
	assert.Equal(t, "string", columnType(&bigquery.FieldSchema{Type: bigquery.StringFieldType}))
	assert.Equal(t, "array<integer>", columnType(&bigquery.FieldSchema{Type: bigquery.IntegerFieldType, Repeated: true}))
}

func TestConvertValueScalars(t *testing.T) {
	assert.Nil(t, convertValue(nil, &bigquery.FieldSchema{Type: bigquery.StringFieldType}))
	assert.Equal(t, "hello", convertValue("hello", &bigquery.FieldSchema{Type: bigquery.StringFieldType}))
	assert.Equal(t, int64(42), convertValue(int64(42), &bigquery.FieldSchema{Type: bigquery.IntegerFieldType}))
}

func TestConvertValueNumeric(t *testing.T) {
	r, ok := (&big.Rat{}).SetString("123.45")
	require.True(t, ok)
	assert.Equal(t, "123.450000000", convertValue(r, &bigquery.FieldSchema{Type: bigquery.NumericFieldType}))

	br, ok := (&big.Rat{}).SetString("1")
	require.True(t, ok)
	got := convertValue(br, &bigquery.FieldSchema{Type: bigquery.BigNumericFieldType}).(string)
	assert.True(t, len(got) > len("1."), "BIGNUMERIC renders with its own, longer scale")
}

func TestConvertValueTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	assert.Equal(t, "2024-01-02T15:04:05Z", convertValue(ts, &bigquery.FieldSchema{Type: bigquery.TimestampFieldType}))
}

func TestConvertValueDate(t *testing.T) {
	d := civil.Date{Year: 2024, Month: 1, Day: 2}
	assert.Equal(t, "2024-01-02", convertValue(d, &bigquery.FieldSchema{Type: bigquery.DateFieldType}))
}

func TestConvertValueBytes(t *testing.T) {
	assert.Equal(t, "aGVsbG8=", convertValue([]byte("hello"), &bigquery.FieldSchema{Type: bigquery.BytesFieldType}))
}

func TestConvertValueRecordFlattensIntoAMap(t *testing.T) {
	schema := []*bigquery.FieldSchema{
		{Name: "city", Type: bigquery.StringFieldType},
		{Name: "zip", Type: bigquery.StringFieldType},
	}
	f := &bigquery.FieldSchema{Type: bigquery.RecordFieldType, Schema: schema}
	record := []bigquery.Value{"London", "SW1"}

	got := convertValue(record, f)
	assert.Equal(t, map[string]any{"city": "London", "zip": "SW1"}, got)
}

func TestConvertValueRepeatedScalar(t *testing.T) {
	f := &bigquery.FieldSchema{Type: bigquery.StringFieldType, Repeated: true}
	got := convertValue([]bigquery.Value{"a", "b"}, f)
	assert.Equal(t, []any{"a", "b"}, got)
}

func TestConvertValueRepeatedRecord(t *testing.T) {
	nested := []*bigquery.FieldSchema{{Name: "n", Type: bigquery.IntegerFieldType}}
	f := &bigquery.FieldSchema{Type: bigquery.RecordFieldType, Repeated: true, Schema: nested}

	got := convertValue([]bigquery.Value{
		[]bigquery.Value{int64(1)},
		[]bigquery.Value{int64(2)},
	}, f)

	assert.Equal(t, []any{
		map[string]any{"n": int64(1)},
		map[string]any{"n": int64(2)},
	}, got)
}

func TestConfigHashIsStableAndDistinguishesConfigs(t *testing.T) {
	a := &Config{ProjectID: "a"}
	b := &Config{ProjectID: "b"}

	assert.Equal(t, configHash(a), configHash(a))
	assert.NotEqual(t, configHash(a), configHash(b))
}

func TestParseConfigDefaultsMaxBytesBilled(t *testing.T) {
	config, err := parseConfig(map[string]any{"project_id": "p", "use_default_credentials": true})
	require.NoError(t, err)
	assert.Equal(t, int64(defaultMaxBytesBilled), config.MaxBytesBilled)

	config, err = parseConfig(map[string]any{"project_id": "p", "use_default_credentials": true, "max_bytes_billed": 0})
	require.NoError(t, err)
	assert.Zero(t, config.MaxBytesBilled, "an explicit zero is kept, falling back to the project's own limit")
}

// Two sources served by one process must each keep their own client; asking
// for one never closes the other.
func TestQueryClientsKeepOneClientPerConfig(t *testing.T) {
	// NewClient builds its HTTP transport lazily and never dials out until a
	// call is made, so this never touches the network.
	open := func() *cachedClient {
		client, err := bigquery.NewClient(context.Background(), "test-project", option.WithoutAuthentication(), option.WithEndpoint("http://127.0.0.1:1"))
		require.NoError(t, err)
		return &cachedClient{client: client}
	}
	var q queryClients
	a, b := q.put("a", open()), q.put("b", open())

	assert.Same(t, a, q.get("a"))
	assert.Same(t, b, q.get("b"))
	assert.Same(t, a, q.put("a", open()), "a client opened concurrently for the same config is discarded")
	assert.NotSame(t, a, b)
}

func TestQueryClientsCloseIdleClients(t *testing.T) {
	client, err := bigquery.NewClient(context.Background(), "test-project", option.WithoutAuthentication(), option.WithEndpoint("http://127.0.0.1:1"))
	require.NoError(t, err)

	var q queryClients
	q.put("a", &cachedClient{client: client})
	q.clients["a"].lastUsed = time.Now().Add(-clientIdleTTL - time.Second)

	assert.Nil(t, q.get("a"), "an idle client is closed and forgotten")
}
