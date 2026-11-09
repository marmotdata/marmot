package oracle

import (
	"database/sql"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Oracle objects are addressed SERVICE.SCHEMA.OBJECT: the schema is what an Oracle user calls the database, and the same table name in two schemas must stay two assets. MRNs are lowercased; the asset Name keeps the case Oracle stores, which is upper case unless the object was quoted.

func testConfig() *Config {
	return &Config{Host: "localhost", Port: 1521, ServiceName: "FREEPDB1"}
}

func TestTableMRN_IsQualifiedByServiceAndSchema(t *testing.T) {
	cfg := testConfig()

	assert.Equal(t, "mrn://asset/table/oracle/freepdb1.hr.employees", assetMRN(cfg, "Table", cfg.serviceID(), "HR", "EMPLOYEES"))
	assert.Equal(t, "mrn://asset/view/oracle/freepdb1.hr.emp_details", assetMRN(cfg, "View", cfg.serviceID(), "HR", "EMP_DETAILS"))
	assert.Equal(t, "mrn://asset/function/oracle/freepdb1.hr.raise_salary", assetMRN(cfg, "Function", cfg.serviceID(), "HR", "RAISE_SALARY"))
}

func TestTableMRN_SameNameInTwoSchemasStaysTwoAssets(t *testing.T) {
	cfg := testConfig()

	assert.NotEqual(t,
		assetMRN(cfg, "Table", cfg.serviceID(), "HR", "AUDIT_LOG"),
		assetMRN(cfg, "Table", cfg.serviceID(), "SALES", "AUDIT_LOG"))
}

func TestServiceID_FallsBackToTheSID(t *testing.T) {
	assert.Equal(t, "ORCL", (&Config{SID: "ORCL"}).serviceID())
	assert.Equal(t, "FREEPDB1", (&Config{SID: "ORCL", ServiceName: "FREEPDB1"}).serviceID())
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := testConfig()
	cfg.Zone = "prod-eu"

	got := assetMRN(cfg, "Table", cfg.serviceID(), "HR", "EMPLOYEES")
	assert.Equal(t, "mrn://asset/table/oracle@prod-eu/freepdb1.hr.employees", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func newTestDiscovery() *discovery {
	return &discovery{
		source:  &Source{config: testConfig()},
		info:    serverInfo{Version: "Oracle Database 23ai", Container: "FREEPDB1", BlockSize: 8192},
		objects: make(map[objectRef]string),
		edges:   make(map[string]struct{}),
	}
}

func TestDatabaseAsset_IsTheSchemaUnderTheService(t *testing.T) {
	a := newTestDiscovery().databaseAsset(schemaRow{Name: "HR"}, 2, 1, 1)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "mrn://asset/database/oracle/freepdb1.hr", *a.MRN)
	assert.Equal(t, "HR", *a.Name, "the name people read is the schema's own name")
	assert.Equal(t, []string{"mrn://asset/database/oracle/hr"}, a.PreviousMRNs)
}

func TestTableAsset_NameIsTheObjectsOwnName(t *testing.T) {
	d := newTestDiscovery()
	table := tableRow{Owner: "HR", Name: "EMPLOYEES", Tablespace: sql.NullString{String: "USERS", Valid: true}}

	a := d.objectAsset("Table", table.Owner, table.Name, d.tableMetadata(table, "table", ""), "")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "EMPLOYEES", *a.Name)
	assert.Equal(t, []string{"Oracle"}, a.Providers)
	assert.Equal(t, "mrn://asset/table/oracle/freepdb1.hr.employees", *a.MRN)
	assert.Equal(t, []string{"mrn://asset/table/oracle/hr.employees"}, a.PreviousMRNs)
}

func TestFunctionAsset_NameIsTheRoutinesOwnName(t *testing.T) {
	a := newTestDiscovery().procedureAsset(procedureRow{Owner: "HR", Name: "RAISE_SALARY", Type: "PROCEDURE", Status: "VALID"})

	require.NotNil(t, a.MRN)
	assert.Equal(t, "Function", a.Type)
	assert.Equal(t, "RAISE_SALARY", *a.Name)
	assert.Equal(t, "mrn://asset/function/oracle/freepdb1.hr.raise_salary", *a.MRN)
	assert.Equal(t, []string{"mrn://asset/function/oracle/hr.raise_salary"}, a.PreviousMRNs)
}

func TestQuotedObjectName_KeepsItsCaseInTheNameButNotTheMRN(t *testing.T) {
	cfg := testConfig()

	assert.Equal(t, "HR.MixedCase", objectName("HR", "MixedCase"))
	assert.Equal(t, "mrn://asset/table/oracle/freepdb1.hr.mixedcase", assetMRN(cfg, "Table", cfg.serviceID(), "HR", "MixedCase"))
}
