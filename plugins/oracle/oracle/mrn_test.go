package oracle

import (
	"database/sql"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Oracle objects are addressed SCHEMA.OBJECT: the schema is what an Oracle
// user calls the database, and the same table name in two schemas must stay
// two assets. mrn.New lowercases the whole MRN; the asset Name keeps the
// case Oracle stores, which is upper case unless the object was quoted.

func TestTableMRN_IsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/oracle/hr.employees", assetMRN("Table", objectName("HR", "EMPLOYEES")))
}

func TestDatabaseMRN_IsTheSchemaName(t *testing.T) {
	assert.Equal(t, "mrn://database/oracle/hr", assetMRN("Database", "HR"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/oracle/hr.emp_details", assetMRN("View", objectName("HR", "EMP_DETAILS")))
}

func TestFunctionMRN_IsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://function/oracle/hr.raise_salary", assetMRN("Function", objectName("HR", "RAISE_SALARY")))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Table", objectName("HR", "EMPLOYEES"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableMRN_AgreesWithTheTrinoConnectorMap(t *testing.T) {
	// plugins/trino names an Oracle table reached through its connector
	// schema + "." + table under the Oracle provider. The two must agree or
	// the same table shows up twice.
	trinoName := func(_, schema, table string) string { return schema + "." + table }

	assert.Equal(t, mrn.New("Table", "Oracle", trinoName("oracle", "HR", "EMPLOYEES")),
		assetMRN("Table", objectName("HR", "EMPLOYEES")))
}

func newTestDiscovery() *discovery {
	return &discovery{
		source:  &Source{config: &Config{Host: "localhost", Port: 1521, ServiceName: "FREEPDB1"}},
		info:    serverInfo{Version: "Oracle Database 23ai", Container: "FREEPDB1", BlockSize: 8192},
		objects: make(map[objectRef]string),
		edges:   make(map[string]struct{}),
	}
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN must be exactly mrn.New over the asset's own fields.
	a := newTestDiscovery().databaseAsset(schemaRow{Name: "HR"}, 2, 1, 1)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://database/oracle/hr", *a.MRN)
	assert.Equal(t, "HR", *a.Name, "the name people read is the schema's own name")
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	d := newTestDiscovery()
	table := tableRow{Owner: "HR", Name: "EMPLOYEES", Tablespace: sql.NullString{String: "USERS", Valid: true}}

	a := d.objectAsset("Table", table.Owner, table.Name, d.tableMetadata(table, "table", ""), "")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "HR.EMPLOYEES", *a.Name)
	assert.Equal(t, []string{"Oracle"}, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://table/oracle/hr.employees", *a.MRN)
}

func TestFunctionAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := newTestDiscovery().procedureAsset(procedureRow{Owner: "HR", Name: "RAISE_SALARY", Type: "PROCEDURE", Status: "VALID"})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "Function", a.Type)
	assert.Equal(t, "HR.RAISE_SALARY", *a.Name)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestDatabaseAsset_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// The container's MRN is not a prefix of its contents: the Contents tree
	// is built from the CONTAINS edges Discover emits, not by matching MRN
	// prefixes.
	db := newTestDiscovery().databaseAsset(schemaRow{Name: "HR"}, 1, 0, 0)
	table := assetMRN("Table", objectName("HR", "EMPLOYEES"))

	assert.Equal(t, "mrn://database/oracle/hr", *db.MRN)
	assert.Equal(t, "mrn://table/oracle/hr.employees", table)
	assert.False(t, len(table) > len(*db.MRN) && table[:len(*db.MRN)] == *db.MRN)
}

func TestQuotedObjectName_KeepsItsCaseInTheNameButNotTheMRN(t *testing.T) {
	name := objectName("HR", "MixedCase")

	assert.Equal(t, "HR.MixedCase", name)
	assert.Equal(t, "mrn://table/oracle/hr.mixedcase", assetMRN("Table", name))
}
