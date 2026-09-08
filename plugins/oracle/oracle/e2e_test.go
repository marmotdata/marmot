package oracle_test

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Oracle database (for
// example gvenzl/oracle-free with APP_USER=marmot). They expect two seeded
// schemas: HR with DEPARTMENTS (2 rows), EMPLOYEES (3 rows, an identity
// primary key, a virtual ANNUAL_SALARY column, a foreign key to
// DEPARTMENTS, table and column comments), the EMP_DETAILS view joining
// both, the DEPT_SALARIES materialized view, the RAISE_SALARY procedure
// and the EMPLOYEE_COUNT function, with DBMS_STATS gathered; and MARMOT
// with one AUDIT_LOG table. The connecting user needs SELECT ANY TABLE and
// EXECUTE ANY PROCEDURE, plus SELECT ANY DICTIONARY for the DBA views test.

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_ORACLE_HOST")
	if host == "" {
		t.Skip("MARMOT_TEST_ORACLE_HOST is not set; skipping Oracle e2e tests")
	}

	port := 1521
	if raw := os.Getenv("MARMOT_TEST_ORACLE_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_ORACLE_PORT must be a number")
		port = parsed
	}

	config := pluginsdk.RawConfig{
		"host":         host,
		"port":         port,
		"user":         envOr("MARMOT_TEST_ORACLE_USER", "marmot"),
		"password":     envOr("MARMOT_TEST_ORACLE_PASSWORD", "Marmot_123"),
		"service_name": envOr("MARMOT_TEST_ORACLE_SERVICE", "FREEPDB1"),
	}
	return config
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

var (
	discoverOnce   sync.Once
	discoverResult *pluginsdk.DiscoveryResult
	discoverErr    error
)

// discover runs one Discover over the wire and shares the result, so the
// small tests below do not each pay for a full dictionary read.
func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	config := e2eConfig(t)
	discoverOnce.Do(func() {
		discoverResult, discoverErr = buildBinary(t).Discover(t.Context(), config)
	})
	require.NoError(t, discoverErr)
	require.NotNil(t, discoverResult)
	return discoverResult
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		a := &result.Assets[i]
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()
	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column schema on %s", *a.Name)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func statisticsOf(result *pluginsdk.DiscoveryResult, assetMRN string) map[string]float64 {
	stats := make(map[string]float64)
	for _, st := range result.Statistics {
		if st.AssetMRN == assetMRN {
			stats[st.MetricName] = st.Value
		}
	}
	return stats
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "oracle", meta.ID)
	assert.Equal(t, "Oracle Database", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "FetchSampleData should be advertised")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "host")

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_ValidateRejectsServiceNameTogetherWithSID(t *testing.T) {
	config := e2eConfig(t)
	config["sid"] = "FREE"

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAValidConfig(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), e2eConfig(t))
	require.NoError(t, err)
}

func TestE2E_DiscoverWrongPasswordFails(t *testing.T) {
	config := e2eConfig(t)
	config["password"] = "definitely-wrong"

	_, err := buildBinary(t).Discover(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_Discover_EmitsADatabaseAssetPerSchema(t *testing.T) {
	result := discover(t)

	hr := findAsset(result, "Database", "HR")
	require.NotNil(t, hr, "expected a Database asset for the HR schema")
	assert.Equal(t, []string{"Oracle"}, hr.Providers)
	assert.Equal(t, "mrn://database/oracle/hr", *hr.MRN)
	assert.Equal(t, "HR", hr.Metadata["schema"])
	assert.Equal(t, "FREEPDB1", hr.Metadata["service_name"])
	assert.Equal(t, "FREEPDB1", hr.Metadata["container"])
	assert.Contains(t, hr.Metadata["oracle_version"], "Oracle")
	assert.EqualValues(t, 2, hr.Metadata["table_count"])
	assert.EqualValues(t, 1, hr.Metadata["view_count"])
	assert.EqualValues(t, 1, hr.Metadata["materialized_view_count"])
	assert.NotEmpty(t, hr.Metadata["created"])

	marmot := findAsset(result, "Database", "MARMOT")
	require.NotNil(t, marmot, "expected a Database asset for the MARMOT schema")
	assert.EqualValues(t, 1, marmot.Metadata["table_count"])
}

func TestE2E_Discover_ExcludesOracleMaintainedSchemas(t *testing.T) {
	result := discover(t)

	assert.Nil(t, findAsset(result, "Database", "SYS"))
	assert.Nil(t, findAsset(result, "Database", "SYSTEM"))
	for _, a := range result.Assets {
		if a.Type == "Table" {
			assert.NotContains(t, *a.Name, "SYS.", "a SYS table leaked into results")
		}
	}
}

func TestE2E_Discover_TablesAreSchemaQualified(t *testing.T) {
	result := discover(t)

	employees := findAsset(result, "Table", "HR.EMPLOYEES")
	require.NotNil(t, employees)
	assert.Equal(t, "mrn://table/oracle/hr.employees", *employees.MRN)
	assert.Equal(t, "HR", employees.Metadata["schema"])
	assert.Equal(t, "EMPLOYEES", employees.Metadata["table_name"])
	assert.Equal(t, "table", employees.Metadata["object_type"])
	assert.Equal(t, "USERS", employees.Metadata["tablespace"])
	assert.Equal(t, false, employees.Metadata["partitioned"])
	assert.Equal(t, false, employees.Metadata["temporary"])
	assert.Equal(t, false, employees.Metadata["iot"])
	assert.EqualValues(t, 3, employees.Metadata["num_rows"])
	assert.NotEmpty(t, employees.Metadata["last_analyzed"])

	assert.NotNil(t, findAsset(result, "Table", "HR.DEPARTMENTS"))
	assert.NotNil(t, findAsset(result, "Table", "MARMOT.AUDIT_LOG"))
}

func TestE2E_Discover_TableCommentBecomesTheDescription(t *testing.T) {
	result := discover(t)

	employees := findAsset(result, "Table", "HR.EMPLOYEES")
	require.NotNil(t, employees)
	require.NotNil(t, employees.Description)
	assert.Equal(t, "Employee master data", *employees.Description)
	assert.Equal(t, "Employee master data", employees.Metadata["comment"])
}

func TestE2E_Discover_MaterializedViewStorageIsNotATable(t *testing.T) {
	result := discover(t)

	assert.Nil(t, findAsset(result, "Table", "HR.DEPT_SALARIES"),
		"the table backing a materialized view must not be catalogued as a table")
}

func TestE2E_Discover_EmployeesColumns(t *testing.T) {
	result := discover(t)

	employees := findAsset(result, "Table", "HR.EMPLOYEES")
	require.NotNil(t, employees)
	columns := columnsOf(t, employees)

	require.Contains(t, columns, "EMPLOYEE_ID")
	assert.Equal(t, true, columns["EMPLOYEE_ID"]["is_primary_key"])
	assert.Equal(t, true, columns["EMPLOYEE_ID"]["is_identity"])
	assert.Equal(t, false, columns["EMPLOYEE_ID"]["is_nullable"])
	assert.Equal(t, "NUMBER", columns["EMPLOYEE_ID"]["data_type"])

	require.Contains(t, columns, "SALARY")
	assert.Equal(t, "NUMBER(10,2)", columns["SALARY"]["data_type"])
	assert.Equal(t, "Monthly salary", columns["SALARY"]["description"])
	assert.Equal(t, true, columns["SALARY"]["is_nullable"])

	require.Contains(t, columns, "LAST_NAME")
	assert.Equal(t, "VARCHAR2(50 CHAR)", columns["LAST_NAME"]["data_type"])

	require.Contains(t, columns, "FIRST_NAME")
	assert.Equal(t, "VARCHAR2(20)", columns["FIRST_NAME"]["data_type"])

	require.Contains(t, columns, "LAST_LOGIN")
	assert.Equal(t, "TIMESTAMP(6) WITH TIME ZONE", columns["LAST_LOGIN"]["data_type"])

	require.Contains(t, columns, "HIRE_DATE")
	assert.Equal(t, "DATE", columns["HIRE_DATE"]["data_type"])
	assert.Equal(t, "SYSDATE", columns["HIRE_DATE"]["default_expression"])

	require.Contains(t, columns, "ANNUAL_SALARY")
	assert.Equal(t, true, columns["ANNUAL_SALARY"]["is_virtual"])
	assert.Equal(t, `"SALARY"*12`, columns["ANNUAL_SALARY"]["default_expression"])

	require.Contains(t, columns, "DEPARTMENT_ID")
	assert.Equal(t, "NUMBER(4)", columns["DEPARTMENT_ID"]["data_type"])
	assert.Nil(t, columns["DEPARTMENT_ID"]["is_primary_key"])
}

func TestE2E_Discover_ForeignKeyEdge(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "mrn://table/oracle/hr.employees", "mrn://table/oracle/hr.departments", "FOREIGN_KEY"),
		"expected HR.EMPLOYEES -> HR.DEPARTMENTS foreign key edge")
}

func TestE2E_Discover_ContainsEdges(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "mrn://database/oracle/hr", "mrn://table/oracle/hr.employees", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://database/oracle/hr", "mrn://view/oracle/hr.emp_details", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://database/oracle/hr", "mrn://function/oracle/hr.raise_salary", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://database/oracle/marmot", "mrn://table/oracle/marmot.audit_log", "CONTAINS"))
}

func TestE2E_Discover_ViewHasQueryAndLineage(t *testing.T) {
	result := discover(t)

	view := findAsset(result, "View", "HR.EMP_DETAILS")
	require.NotNil(t, view)
	assert.Equal(t, "mrn://view/oracle/hr.emp_details", *view.MRN)
	assert.Equal(t, "view", view.Metadata["object_type"])
	require.NotNil(t, view.Query)
	assert.Contains(t, *view.Query, "FROM employees")
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "SQL", *view.QueryLanguage)

	columns := columnsOf(t, view)
	assert.Contains(t, columns, "DEPARTMENT_NAME")

	// VIEW_OF runs base table -> view, the direction the data flows.
	assert.True(t, hasEdge(result, "mrn://table/oracle/hr.employees", "mrn://view/oracle/hr.emp_details", "VIEW_OF"))
	assert.True(t, hasEdge(result, "mrn://table/oracle/hr.departments", "mrn://view/oracle/hr.emp_details", "VIEW_OF"))
}

func TestE2E_Discover_MaterializedView(t *testing.T) {
	result := discover(t)

	mview := findAsset(result, "View", "HR.DEPT_SALARIES")
	require.NotNil(t, mview)
	assert.Equal(t, "materialized_view", mview.Metadata["object_type"])
	assert.Equal(t, true, mview.Metadata["materialized"])
	assert.Equal(t, "DEMAND", mview.Metadata["refresh_mode"])
	assert.Equal(t, "COMPLETE", mview.Metadata["refresh_method"])
	assert.Equal(t, "IMMEDIATE", mview.Metadata["build_mode"])
	assert.NotEmpty(t, mview.Metadata["last_refresh"])
	assert.NotEmpty(t, mview.Metadata["staleness"])
	require.NotNil(t, mview.Query)
	assert.Contains(t, *mview.Query, "SUM(salary)")

	assert.True(t, hasEdge(result, "mrn://table/oracle/hr.employees", "mrn://view/oracle/hr.dept_salaries", "VIEW_OF"))
}

func TestE2E_Discover_FunctionAssets(t *testing.T) {
	result := discover(t)

	procedure := findAsset(result, "Function", "HR.RAISE_SALARY")
	require.NotNil(t, procedure)
	assert.Equal(t, "mrn://function/oracle/hr.raise_salary", *procedure.MRN)
	assert.Equal(t, "procedure", procedure.Metadata["object_type"])
	assert.Equal(t, "VALID", procedure.Metadata["status"])
	assert.NotEmpty(t, procedure.Metadata["created"])
	assert.NotEmpty(t, procedure.Metadata["last_ddl_time"])

	function := findAsset(result, "Function", "HR.EMPLOYEE_COUNT")
	require.NotNil(t, function)
	assert.Equal(t, "function", function.Metadata["object_type"])
}

func TestE2E_Discover_Statistics(t *testing.T) {
	result := discover(t)

	stats := statisticsOf(result, "mrn://table/oracle/hr.employees")
	assert.Equal(t, float64(3), stats["asset.row_count"])
	assert.Equal(t, float64(9), stats["asset.column_count"])
	// 5 blocks of the default 8 KiB block size after DBMS_STATS ran.
	assert.Equal(t, float64(5*8192), stats["asset.size_bytes"])

	viewStats := statisticsOf(result, "mrn://view/oracle/hr.emp_details")
	assert.Equal(t, float64(4), viewStats["asset.column_count"])
	assert.NotContains(t, viewStats, "asset.row_count", "views have no row estimate")
}

func TestE2E_Discover_EveryMRNMatchesTheServersDerivation(t *testing.T) {
	result := discover(t)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestE2E_Discover_EveryEdgeEndpointExists(t *testing.T) {
	result := discover(t)

	known := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		known[*a.MRN] = struct{}{}
	}
	for _, e := range result.Lineage {
		assert.Contains(t, known, e.Source, "edge source is not an asset of this run")
		assert.Contains(t, known, e.Target, "edge target is not an asset of this run")
	}
}

func TestE2E_Discover_SchemasFilterLimitsTheRun(t *testing.T) {
	config := e2eConfig(t)
	config["schemas"] = []interface{}{"hr"}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "Database", "HR"))
	assert.Nil(t, findAsset(result, "Database", "MARMOT"))
	assert.Nil(t, findAsset(result, "Table", "MARMOT.AUDIT_LOG"))
}

func TestE2E_Discover_DBAViewsSeeTheSameObjects(t *testing.T) {
	config := e2eConfig(t)
	config["use_dba_views"] = true
	config["schemas"] = []interface{}{"HR"}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "Table", "HR.EMPLOYEES"))
	assert.NotNil(t, findAsset(result, "View", "HR.EMP_DETAILS"))
	assert.NotNil(t, findAsset(result, "Function", "HR.RAISE_SALARY"))
	assert.True(t, hasEdge(result, "mrn://table/oracle/hr.employees", "mrn://table/oracle/hr.departments", "FOREIGN_KEY"))
}

func TestE2E_Discover_FlagsTurnObjectKindsOff(t *testing.T) {
	config := e2eConfig(t)
	config["schemas"] = []interface{}{"HR"}
	config["include_views"] = false
	config["include_materialized_views"] = false
	config["include_procedures"] = false
	config["include_statistics"] = false
	config["include_columns"] = false

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotNil(t, findAsset(result, "Table", "HR.EMPLOYEES"))
	assert.Nil(t, findAsset(result, "View", "HR.EMP_DETAILS"))
	assert.Nil(t, findAsset(result, "View", "HR.DEPT_SALARIES"))
	assert.Nil(t, findAsset(result, "Table", "HR.DEPT_SALARIES"), "the storage table stays hidden even when materialized views are off")
	assert.Nil(t, findAsset(result, "Function", "HR.RAISE_SALARY"))
	assert.Empty(t, result.Statistics)
	assert.Empty(t, findAsset(result, "Table", "HR.EMPLOYEES").Schema)
}

func TestE2E_FetchSampleData(t *testing.T) {
	result := discover(t)
	employees := findAsset(result, "Table", "HR.EMPLOYEES")
	require.NotNil(t, employees)

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), e2eConfig(t), employees)
	require.NoError(t, err)

	assert.Contains(t, columns, "EMPLOYEE_ID")
	assert.Contains(t, columns, "SALARY")
	assert.Contains(t, columns, "LAST_LOGIN")
	require.Len(t, rows, 3)

	byColumn := func(row []interface{}, name string) interface{} {
		for i, c := range columns {
			if c == name {
				return row[i]
			}
		}
		return nil
	}
	assert.Equal(t, "Alice", byColumn(rows[0], "FIRST_NAME"))
	// Numbers travel as JSON numbers, dates as RFC 3339 strings.
	assert.EqualValues(t, 5000.5, byColumn(rows[0], "SALARY"))
	assert.EqualValues(t, 1, byColumn(rows[0], "EMPLOYEE_ID"))
	assert.Contains(t, byColumn(rows[0], "HIRE_DATE"), "T")
	assert.Nil(t, byColumn(rows[1], "LAST_LOGIN"))
}

func TestE2E_FetchSampleData_FallsBackToTheNameWhenMetadataIsMissing(t *testing.T) {
	name := "HR.DEPARTMENTS"
	asset := &pluginsdk.Asset{Name: &name, Type: "Table", Metadata: map[string]interface{}{}}

	columns, rows, err := buildBinary(t).FetchSampleData(t.Context(), e2eConfig(t), asset)
	require.NoError(t, err)

	assert.Equal(t, []string{"DEPARTMENT_ID", "DEPARTMENT_NAME", "LOCATION"}, columns)
	assert.Len(t, rows, 2)
}
