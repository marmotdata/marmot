package oracle

import (
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseURL splits a go-ora connection string into its parts. go-ora renders
// the options from a map, so their order is not stable and the tests look
// at the parsed query instead of the raw string.
func parseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestBuildConnectionURL_UsesTheServiceNameAsThePath(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 1521, User: "marmot", Password: "pw", ServiceName: "FREEPDB1"}))

	assert.Equal(t, "oracle", u.Scheme)
	assert.Equal(t, "db.example.com:1521", u.Host)
	assert.Equal(t, "/FREEPDB1", u.Path)
	assert.Equal(t, "marmot", u.User.Username())
	password, _ := u.User.Password()
	assert.Equal(t, "pw", password)
}

func TestBuildConnectionURL_SIDIsAnOptionNotAPath(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 1521, User: "marmot", Password: "pw", SID: "ORCL"}))

	assert.Equal(t, "/", u.Path)
	assert.Equal(t, "ORCL", u.Query().Get("SID"))
}

func TestBuildConnectionURL_EscapesThePassword(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 1521, User: "marmot", Password: "p@ss/w:rd", ServiceName: "FREEPDB1"}))

	password, _ := u.User.Password()
	assert.Equal(t, "p@ss/w:rd", password)
	assert.Equal(t, "db.example.com:1521", u.Host)
}

func TestBuildConnectionURL_SetsAConnectTimeout(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 1521, User: "marmot", Password: "pw", ServiceName: "FREEPDB1"}))

	assert.Equal(t, "15", u.Query().Get("CONNECT TIMEOUT"))
}

func TestBuildConnectionURL_PlainConnectionHasNoSSLOptions(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 1521, User: "marmot", Password: "pw", ServiceName: "FREEPDB1", SSLVerify: true}))

	assert.False(t, u.Query().Has("SSL"))
	assert.False(t, u.Query().Has("SSL VERIFY"))
	assert.False(t, u.Query().Has("WALLET"))
}

func TestBuildConnectionURL_SSLTurnsOnTCPS(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 2484, User: "marmot", Password: "pw", ServiceName: "FREEPDB1", SSL: true, SSLVerify: true}))

	assert.Equal(t, "true", u.Query().Get("SSL"))
	assert.Equal(t, "true", u.Query().Get("SSL VERIFY"))
}

func TestBuildConnectionURL_SSLVerifyCanBeTurnedOff(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 2484, User: "marmot", Password: "pw", ServiceName: "FREEPDB1", SSL: true, SSLVerify: false}))

	assert.Equal(t, "false", u.Query().Get("SSL VERIFY"))
}

func TestBuildConnectionURL_WalletPathIsPassedThrough(t *testing.T) {
	u := parseURL(t, buildConnectionURL(&Config{Host: "db.example.com", Port: 2484, User: "marmot", Password: "pw", ServiceName: "FREEPDB1", WalletPath: "/opt/oracle/wallet"}))

	assert.Equal(t, "/opt/oracle/wallet", u.Query().Get("WALLET"))
}

func TestNormaliseSchemaNames_UpperCasesBareNames(t *testing.T) {
	assert.Equal(t, []string{"HR", "SALES"}, normaliseSchemaNames([]string{"hr", " Sales "}))
}

func TestNormaliseSchemaNames_QuotedNamesKeepTheirCase(t *testing.T) {
	assert.Equal(t, []string{"MixedCase"}, normaliseSchemaNames([]string{`"MixedCase"`}))
}

func TestNormaliseSchemaNames_DropsEmptyAndDuplicateEntries(t *testing.T) {
	assert.Equal(t, []string{"HR"}, normaliseSchemaNames([]string{"hr", "", "HR", "  "}))
}

func TestNormaliseSchemaNames_NilStaysNil(t *testing.T) {
	assert.Nil(t, normaliseSchemaNames(nil))
}

func nullInt(v int64) sql.NullInt64      { return sql.NullInt64{Int64: v, Valid: true} }
func nullString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }

func TestRenderDataType_NumberWithPrecisionAndScale(t *testing.T) {
	assert.Equal(t, "NUMBER(10,2)", renderDataType(columnRow{DataType: "NUMBER", Precision: nullInt(10), Scale: nullInt(2)}))
}

func TestRenderDataType_NumberWithZeroScaleShowsPrecisionOnly(t *testing.T) {
	assert.Equal(t, "NUMBER(4)", renderDataType(columnRow{DataType: "NUMBER", Precision: nullInt(4), Scale: nullInt(0)}))
}

func TestRenderDataType_NumberWithoutPrecisionIsBare(t *testing.T) {
	assert.Equal(t, "NUMBER", renderDataType(columnRow{DataType: "NUMBER"}))
}

func TestRenderDataType_IntegerIsNumber38(t *testing.T) {
	// INTEGER is stored as NUMBER with no precision and a zero scale.
	assert.Equal(t, "NUMBER(38)", renderDataType(columnRow{DataType: "NUMBER", Scale: nullInt(0)}))
}

func TestRenderDataType_NumberWithScaleOnly(t *testing.T) {
	assert.Equal(t, "NUMBER(*,2)", renderDataType(columnRow{DataType: "NUMBER", Scale: nullInt(2)}))
}

func TestRenderDataType_FloatCarriesItsBinaryPrecision(t *testing.T) {
	assert.Equal(t, "FLOAT(126)", renderDataType(columnRow{DataType: "FLOAT", Precision: nullInt(126)}))
}

func TestRenderDataType_Varchar2WithByteSemantics(t *testing.T) {
	assert.Equal(t, "VARCHAR2(20)", renderDataType(columnRow{DataType: "VARCHAR2", DataLength: nullInt(20), CharLength: nullInt(20), CharUsed: nullString("B")}))
}

func TestRenderDataType_Varchar2WithCharSemantics(t *testing.T) {
	// A 50 CHAR column takes 200 bytes in a UTF-8 database; the type shows
	// the declared 50 characters, not the storage.
	assert.Equal(t, "VARCHAR2(50 CHAR)", renderDataType(columnRow{DataType: "VARCHAR2", DataLength: nullInt(200), CharLength: nullInt(50), CharUsed: nullString("C")}))
}

func TestRenderDataType_Char(t *testing.T) {
	assert.Equal(t, "CHAR(1)", renderDataType(columnRow{DataType: "CHAR", DataLength: nullInt(1), CharLength: nullInt(1), CharUsed: nullString("B")}))
}

func TestRenderDataType_NationalTypesAlwaysCountCharacters(t *testing.T) {
	assert.Equal(t, "NVARCHAR2(10)", renderDataType(columnRow{DataType: "NVARCHAR2", DataLength: nullInt(20), CharLength: nullInt(10), CharUsed: nullString("C")}))
}

func TestRenderDataType_Raw(t *testing.T) {
	assert.Equal(t, "RAW(16)", renderDataType(columnRow{DataType: "RAW", DataLength: nullInt(16)}))
}

func TestRenderDataType_TimestampWithTimeZoneIsVerbatim(t *testing.T) {
	assert.Equal(t, "TIMESTAMP(6) WITH TIME ZONE", renderDataType(columnRow{DataType: "TIMESTAMP(6) WITH TIME ZONE", DataLength: nullInt(13), Scale: nullInt(6)}))
}

func TestRenderDataType_DateAndLobsAreVerbatim(t *testing.T) {
	assert.Equal(t, "DATE", renderDataType(columnRow{DataType: "DATE", DataLength: nullInt(7)}))
	assert.Equal(t, "CLOB", renderDataType(columnRow{DataType: "CLOB", DataLength: nullInt(4000)}))
}

func TestExtractViewReferences_SimpleFrom(t *testing.T) {
	refs := extractViewReferences("SELECT id FROM employees")

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}}, refs)
}

func TestExtractViewReferences_JoinsAndAliases(t *testing.T) {
	refs := extractViewReferences(`SELECT e.employee_id, d.department_name
  FROM employees e
  JOIN departments d ON e.department_id = d.department_id
  LEFT OUTER JOIN locations AS l ON d.location_id = l.location_id`)

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}, {Name: "DEPARTMENTS"}, {Name: "LOCATIONS"}}, refs)
}

func TestExtractViewReferences_CommaSeparatedFromList(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM employees e, departments d, jobs WHERE e.department_id = d.department_id")

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}, {Name: "DEPARTMENTS"}, {Name: "JOBS"}}, refs)
}

func TestExtractViewReferences_QualifiedByOwner(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM hr.employees JOIN sales.orders o ON o.emp_id = employees.employee_id")

	assert.Equal(t, []objectRef{{Owner: "HR", Name: "EMPLOYEES"}, {Owner: "SALES", Name: "ORDERS"}}, refs)
}

func TestExtractViewReferences_QuotedIdentifiersKeepTheirCase(t *testing.T) {
	refs := extractViewReferences(`SELECT * FROM "Hr"."MixedCase" JOIN "Other" ON 1 = 1`)

	assert.Equal(t, []objectRef{{Owner: "Hr", Name: "MixedCase"}, {Name: "Other"}}, refs)
}

func TestExtractViewReferences_SubqueryIsNotAnObject(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM (SELECT * FROM employees) e JOIN departments d ON 1 = 1")

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}, {Name: "DEPARTMENTS"}}, refs)
}

func TestExtractViewReferences_IgnoresCommentsAndStringLiterals(t *testing.T) {
	refs := extractViewReferences(`SELECT 'FROM fake' AS label -- FROM comment_table
  /* FROM block_table */
  FROM employees`)

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}}, refs)
}

func TestExtractViewReferences_CommonTableExpressionsAreNotObjects(t *testing.T) {
	refs := extractViewReferences(`WITH recent AS (SELECT * FROM employees WHERE hire_date > SYSDATE - 30),
  big AS (SELECT * FROM departments)
  SELECT * FROM recent r JOIN big b ON r.department_id = b.department_id`)

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}, {Name: "DEPARTMENTS"}}, refs)
}

func TestExtractViewReferences_DatabaseLinksAreNotLocalObjects(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM employees@remote_db r, departments d")

	assert.Equal(t, []objectRef{{Name: "DEPARTMENTS"}}, refs)
}

func TestExtractViewReferences_DeduplicatesRepeatedObjects(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM employees UNION ALL SELECT * FROM employees")

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}}, refs)
}

func TestExtractViewReferences_TableFunctionIsNotAnObject(t *testing.T) {
	refs := extractViewReferences("SELECT * FROM TABLE(split_string('a,b')) t JOIN employees e ON 1 = 1")

	assert.Equal(t, []objectRef{{Name: "EMPLOYEES"}}, refs)
}

func TestExtractViewReferences_EmptyTextHasNoReferences(t *testing.T) {
	assert.Empty(t, extractViewReferences(""))
}

func TestPrimaryKeyColumns_GroupsByTable(t *testing.T) {
	keys := primaryKeyColumns([]constraintRow{
		{Owner: "HR", Table: "EMPLOYEES", Name: "EMP_PK", Type: "P", Column: "EMPLOYEE_ID"},
		{Owner: "HR", Table: "EMPLOYEES", Name: "EMP_EMAIL_UK", Type: "U", Column: "EMAIL"},
		{Owner: "HR", Table: "JOB_HISTORY", Name: "JH_PK", Type: "P", Column: "EMPLOYEE_ID", Position: nullInt(1)},
		{Owner: "HR", Table: "JOB_HISTORY", Name: "JH_PK", Type: "P", Column: "START_DATE", Position: nullInt(2)},
	})

	assert.Equal(t, map[string]bool{"EMPLOYEE_ID": true}, keys["EMPLOYEES"])
	assert.Equal(t, map[string]bool{"EMPLOYEE_ID": true, "START_DATE": true}, keys["JOB_HISTORY"])
	assert.False(t, keys["EMPLOYEES"]["EMAIL"], "a unique key is not the primary key")
}

func TestResolveForeignKeys_ResolvesTheTableThroughTheReferencedConstraint(t *testing.T) {
	keys := resolveForeignKeys([]constraintRow{
		{Owner: "HR", Table: "DEPARTMENTS", Name: "DEPT_PK", Type: "P", Column: "DEPARTMENT_ID"},
		{Owner: "HR", Table: "EMPLOYEES", Name: "EMP_DEPT_FK", Type: "R", Column: "DEPARTMENT_ID",
			ROwner: nullString("HR"), RConstraint: nullString("DEPT_PK"), DeleteRule: nullString("SET NULL")},
	})

	require.Len(t, keys, 1)
	assert.Equal(t, foreignKey{
		Constraint:  "EMP_DEPT_FK",
		SourceOwner: "HR",
		SourceTable: "EMPLOYEES",
		TargetOwner: "HR",
		TargetTable: "DEPARTMENTS",
		DeleteRule:  "SET NULL",
	}, keys[0])
}

func TestResolveForeignKeys_ReferenceToAUniqueKeyResolvesToo(t *testing.T) {
	keys := resolveForeignKeys([]constraintRow{
		{Owner: "HR", Table: "EMPLOYEES", Name: "EMP_EMAIL_UK", Type: "U", Column: "EMAIL"},
		{Owner: "HR", Table: "LOGINS", Name: "LOGIN_EMAIL_FK", Type: "R", Column: "EMAIL",
			ROwner: nullString("HR"), RConstraint: nullString("EMP_EMAIL_UK")},
	})

	require.Len(t, keys, 1)
	assert.Equal(t, "EMPLOYEES", keys[0].TargetTable)
}

func TestResolveForeignKeys_CrossesSchemas(t *testing.T) {
	keys := resolveForeignKeys([]constraintRow{
		{Owner: "HR", Table: "DEPARTMENTS", Name: "DEPT_PK", Type: "P", Column: "DEPARTMENT_ID"},
		{Owner: "SALES", Table: "ORDERS", Name: "ORD_DEPT_FK", Type: "R", Column: "DEPARTMENT_ID",
			ROwner: nullString("HR"), RConstraint: nullString("DEPT_PK")},
	})

	require.Len(t, keys, 1)
	assert.Equal(t, "SALES", keys[0].SourceOwner)
	assert.Equal(t, "HR", keys[0].TargetOwner)
	assert.Equal(t, "DEPARTMENTS", keys[0].TargetTable)
}

func TestResolveForeignKeys_DropsReferencesIntoSchemasNotRead(t *testing.T) {
	keys := resolveForeignKeys([]constraintRow{
		{Owner: "HR", Table: "EMPLOYEES", Name: "EMP_REGION_FK", Type: "R", Column: "REGION_ID",
			ROwner: nullString("GEO"), RConstraint: nullString("REGION_PK")},
	})

	assert.Empty(t, keys)
}

func TestResolveForeignKeys_OneKeyPerMultiColumnConstraint(t *testing.T) {
	keys := resolveForeignKeys([]constraintRow{
		{Owner: "HR", Table: "JOB_HISTORY", Name: "JH_PK", Type: "P", Column: "EMPLOYEE_ID", Position: nullInt(1)},
		{Owner: "HR", Table: "JOB_HISTORY", Name: "JH_PK", Type: "P", Column: "START_DATE", Position: nullInt(2)},
		{Owner: "HR", Table: "REVIEWS", Name: "REV_JH_FK", Type: "R", Column: "EMPLOYEE_ID", Position: nullInt(1),
			ROwner: nullString("HR"), RConstraint: nullString("JH_PK")},
		{Owner: "HR", Table: "REVIEWS", Name: "REV_JH_FK", Type: "R", Column: "START_DATE", Position: nullInt(2),
			ROwner: nullString("HR"), RConstraint: nullString("JH_PK")},
	})

	require.Len(t, keys, 1)
	assert.Equal(t, "JOB_HISTORY", keys[0].TargetTable)
}

func TestSplitObjectName_SplitsOnTheFirstDot(t *testing.T) {
	schema, object := splitObjectName("HR.EMPLOYEES")
	assert.Equal(t, "HR", schema)
	assert.Equal(t, "EMPLOYEES", object)
}

func TestSplitObjectName_UnqualifiedNameHasNoSchema(t *testing.T) {
	schema, object := splitObjectName("EMPLOYEES")
	assert.Equal(t, "", schema)
	assert.Equal(t, "EMPLOYEES", object)
}

func TestQuoteIdentifier_DoublesEmbeddedQuotes(t *testing.T) {
	assert.Equal(t, `"HR"`, quoteIdentifier("HR"))
	assert.Equal(t, `"Odd""Name"`, quoteIdentifier(`Odd"Name`))
}

func TestConvertValue_NumberStringsBecomeNumbers(t *testing.T) {
	// go-ora hands NUMBER columns over as strings so nothing is lost.
	assert.Equal(t, int64(42), convertValue("42", "NUMBER"))
	assert.Equal(t, 5000.5, convertValue("5000.5", "NUMBER"))
}

func TestConvertValue_NumberTooBigForInt64StaysAString(t *testing.T) {
	huge := "123456789012345678901234567890"
	assert.Equal(t, 1.2345678901234568e29, convertValue(huge, "NUMBER"))
}

func TestConvertValue_TextStringsAreLeftAlone(t *testing.T) {
	assert.Equal(t, "42", convertValue("42", "NCHAR"))
}

func TestConvertValue_TimesBecomeRFC3339(t *testing.T) {
	ts := time.Date(2026, 9, 7, 21, 18, 57, 768729000, time.UTC)
	assert.Equal(t, "2026-09-07T21:18:57.768729Z", convertValue(ts, "DATE"))
}

func TestConvertValue_BinaryBecomesHexUnlessItIsText(t *testing.T) {
	assert.Equal(t, "hello", convertValue([]byte("hello"), "RAW"))
	assert.Equal(t, "0xff00", convertValue([]byte{0xff, 0x00}, "RAW"))
}

func TestConvertValue_NilStaysNil(t *testing.T) {
	assert.Nil(t, convertValue(nil, "NUMBER"))
}
