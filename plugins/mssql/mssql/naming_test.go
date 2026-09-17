package mssql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Data type rendering. The numbers below are the ones sys.columns really
// reports, copied from an Azure SQL Edge instance.

func TestRenderDataType_VarcharKeepsItsByteLength(t *testing.T) {
	assert.Equal(t, "varchar(255)", renderDataType("varchar", 255, 0, 0))
}

func TestRenderDataType_NvarcharHalvesTheByteLength(t *testing.T) {
	// SQL Server stores nvarchar as UTF-16, so max_length is twice the
	// declared character length: nvarchar(100) reports 200.
	assert.Equal(t, "nvarchar(100)", renderDataType("nvarchar", 200, 0, 0))
}

func TestRenderDataType_NvarcharMaxIsNotHalved(t *testing.T) {
	assert.Equal(t, "nvarchar(max)", renderDataType("nvarchar", -1, 0, 0))
}

func TestRenderDataType_VarcharMaxIsNamedMax(t *testing.T) {
	assert.Equal(t, "varchar(max)", renderDataType("varchar", -1, 0, 0))
}

func TestRenderDataType_VarbinaryMaxIsNamedMax(t *testing.T) {
	assert.Equal(t, "varbinary(max)", renderDataType("varbinary", -1, 0, 0))
}

func TestRenderDataType_NcharHalvesTheByteLength(t *testing.T) {
	assert.Equal(t, "nchar(10)", renderDataType("nchar", 20, 0, 0))
}

func TestRenderDataType_CharKeepsItsByteLength(t *testing.T) {
	assert.Equal(t, "char(3)", renderDataType("char", 3, 0, 0))
}

func TestRenderDataType_DecimalCarriesPrecisionAndScale(t *testing.T) {
	assert.Equal(t, "decimal(10,2)", renderDataType("decimal", 9, 10, 2))
}

func TestRenderDataType_NumericCarriesPrecisionAndScale(t *testing.T) {
	assert.Equal(t, "numeric(18,4)", renderDataType("numeric", 9, 18, 4))
}

func TestRenderDataType_Datetime2CarriesOnlyItsScale(t *testing.T) {
	assert.Equal(t, "datetime2(7)", renderDataType("datetime2", 8, 27, 7))
}

func TestRenderDataType_TimeCarriesOnlyItsScale(t *testing.T) {
	assert.Equal(t, "time(3)", renderDataType("time", 5, 13, 3))
}

func TestRenderDataType_IntHasNoSize(t *testing.T) {
	assert.Equal(t, "int", renderDataType("int", 4, 10, 0))
}

func TestRenderDataType_UniqueidentifierHasNoSize(t *testing.T) {
	assert.Equal(t, "uniqueidentifier", renderDataType("uniqueidentifier", 16, 0, 0))
}

func TestRenderDataType_LowercasesTheTypeName(t *testing.T) {
	assert.Equal(t, "int", renderDataType("INT", 4, 10, 0))
}

// Bracket quoting.

func TestQuoteIdent_WrapsAPlainName(t *testing.T) {
	assert.Equal(t, "[orders]", quoteIdent("orders"))
}

func TestQuoteIdent_WrapsANameWithASpace(t *testing.T) {
	assert.Equal(t, "[order items]", quoteIdent("order items"))
}

func TestQuoteIdent_DoublesAClosingBracket(t *testing.T) {
	// A closing bracket would otherwise end the quoted name early, which is
	// how an identifier turns into injected SQL.
	assert.Equal(t, "[weird]]name]", quoteIdent("weird]name"))
}

func TestUnquoteIdent_IsTheInverseOfQuoteIdent(t *testing.T) {
	assert.Equal(t, "weird]name", unquoteIdent(quoteIdent("weird]name")))
}

func TestUnquoteIdent_LeavesABareNameAlone(t *testing.T) {
	assert.Equal(t, "orders", unquoteIdent("orders"))
}

// Name building.

func TestQualifiedName_JoinsThreeParts(t *testing.T) {
	assert.Equal(t, "shop.dbo.orders", qualifiedName("shop", "dbo", "orders"))
}

// Routine type naming.

func TestRoutineObjectType_NamesAStoredProcedure(t *testing.T) {
	assert.Equal(t, "stored_procedure", routineObjectType("SQL_STORED_PROCEDURE"))
}

func TestRoutineObjectType_NamesAScalarFunction(t *testing.T) {
	assert.Equal(t, "scalar_function", routineObjectType("SQL_SCALAR_FUNCTION"))
}

func TestRoutineObjectType_NamesAnInlineTableFunction(t *testing.T) {
	assert.Equal(t, "inline_table_function", routineObjectType("SQL_INLINE_TABLE_VALUED_FUNCTION"))
}

func TestRoutineObjectType_NamesATableFunction(t *testing.T) {
	assert.Equal(t, "table_function", routineObjectType("SQL_TABLE_VALUED_FUNCTION"))
}

func TestRoutineObjectType_FallsBackToTheLowercasedTypeDesc(t *testing.T) {
	assert.Equal(t, "clr_stored_procedure", routineObjectType("CLR_STORED_PROCEDURE"))
}

// View reference extraction. The definitions below are the real text
// sys.sql_modules returns, which always includes the CREATE VIEW header.

func TestExtractViewReferences_FindsATwoPartName(t *testing.T) {
	definition := `CREATE VIEW dbo.order_totals AS SELECT * FROM dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_FillsInTheDefaultSchemaForABareName(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_KeepsAThreePartName(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM analytics.dbo.daily_totals`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"analytics.dbo.daily_totals"}, refs)
}

func TestExtractViewReferences_FindsBothSidesOfAJoin(t *testing.T) {
	definition := `CREATE VIEW dbo.order_totals AS
		SELECT c.customer_id, SUM(o.total) AS total_spend
		FROM dbo.customers c
		JOIN dbo.orders o ON o.customer_id = c.customer_id
		GROUP BY c.customer_id`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.ElementsMatch(t, []string{"shop.dbo.customers", "shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_UnwrapsBracketQuotedNames(t *testing.T) {
	definition := `CREATE VIEW [dbo].[v] AS SELECT * FROM [shop].[dbo].[order items]`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.order items"}, refs)
}

func TestExtractViewReferences_HandlesAMixOfQuotedAndBareParts(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM sales.[regions]`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.sales.regions"}, refs)
}

func TestExtractViewReferences_IgnoresCommonTableExpressionNames(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS
		WITH recent AS (SELECT * FROM dbo.orders)
		SELECT * FROM recent`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
	assert.NotContains(t, refs, "shop.dbo.recent")
}

func TestExtractViewReferences_IgnoresEverySchemaNameInAMultiCTEQuery(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS
		WITH a AS (SELECT * FROM dbo.orders),
		     b AS (SELECT * FROM dbo.customers)
		SELECT * FROM a JOIN b ON a.customer_id = b.customer_id`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.ElementsMatch(t, []string{"shop.dbo.orders", "shop.dbo.customers"}, refs)
}

func TestExtractViewReferences_IgnoresASubquery(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM (SELECT * FROM dbo.orders) x`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_IgnoresATableValuedFunctionCall(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM dbo.orders_for_customer(1)`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Empty(t, refs)
}

func TestExtractViewReferences_IgnoresATableVariable(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM @rows`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Empty(t, refs)
}

func TestExtractViewReferences_IgnoresATempTable(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT * FROM #staging`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Empty(t, refs)
}

func TestExtractViewReferences_IgnoresAFourPartLinkedServerName(t *testing.T) {
	// A four-part name addresses another instance, which this run cannot see.
	definition := `CREATE VIEW dbo.v AS SELECT * FROM remote.shop.dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Empty(t, refs)
}

func TestExtractViewReferences_IgnoresNamesInsideALineComment(t *testing.T) {
	definition := "CREATE VIEW dbo.v AS\n-- FROM dbo.secret\nSELECT * FROM dbo.orders"

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_IgnoresNamesInsideABlockComment(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS /* FROM dbo.secret */ SELECT * FROM dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_IgnoresNamesInsideAStringLiteral(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT 'from dbo.secret' AS note FROM dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_ReturnsEachObjectOnce(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS
		SELECT * FROM dbo.orders
		UNION ALL
		SELECT * FROM dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_MatchesCaseInsensitively(t *testing.T) {
	definition := `create view dbo.v as select * from dbo.orders`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Equal(t, []string{"shop.dbo.orders"}, refs)
}

func TestExtractViewReferences_ReturnsNothingForADefinitionWithNoSource(t *testing.T) {
	definition := `CREATE VIEW dbo.v AS SELECT 1 AS one`

	refs := extractViewReferences(definition, "shop", "dbo")

	assert.Empty(t, refs)
}

// Value conversion for sample data.

func TestConvertValue_TurnsAUniqueidentifierIntoItsTextForm(t *testing.T) {
	// The 16 bytes below are what the wire carries for
	// 17C9F487-14D3-41AE-B541-F372EE2A5659: the first three groups arrive
	// byte-reversed, which is why the raw bytes cannot just be hex printed.
	raw := []byte{0x87, 0xf4, 0xc9, 0x17, 0xd3, 0x14, 0xae, 0x41, 0xb5, 0x41, 0xf3, 0x72, 0xee, 0x2a, 0x56, 0x59}

	assert.Equal(t, "17C9F487-14D3-41AE-B541-F372EE2A5659", convertValue(raw, "UNIQUEIDENTIFIER"))
}

func TestConvertValue_KeepsADecimalAsExactText(t *testing.T) {
	// Going through float64 would turn 99.99 into 99.98999999999999.
	assert.Equal(t, "99.99", convertValue([]byte("99.99"), "DECIMAL"))
}

func TestConvertValue_HexEncodesVarbinary(t *testing.T) {
	assert.Equal(t, "0xdeadbeef", convertValue([]byte{0xde, 0xad, 0xbe, 0xef}, "VARBINARY"))
}

func TestConvertValue_LeavesNilAlone(t *testing.T) {
	assert.Nil(t, convertValue(nil, "INT"))
}

func TestConvertValue_PassesAnIntegerThrough(t *testing.T) {
	assert.Equal(t, int64(42), convertValue(int64(42), "INT"))
}

// Extended properties and identity values arrive as sql_variant, so the
// driver hands them back untyped.

func TestSQLVariantString_ReadsAString(t *testing.T) {
	assert.Equal(t, "People who buy things", sqlVariantString("People who buy things"))
}

func TestSQLVariantString_ReadsRawBytes(t *testing.T) {
	assert.Equal(t, "a comment", sqlVariantString([]byte("a comment")))
}

func TestSQLVariantString_TurnsAbsentIntoEmpty(t *testing.T) {
	assert.Equal(t, "", sqlVariantString(nil))
}

func TestSQLVariantInt_ReadsAnInt64(t *testing.T) {
	value, ok := sqlVariantInt(int64(1))

	require.True(t, ok)
	assert.Equal(t, int64(1), value)
}

func TestSQLVariantInt_ReadsANarrowerInteger(t *testing.T) {
	// A smallint identity column reports its seed as an int16.
	value, ok := sqlVariantInt(int16(5))

	require.True(t, ok)
	assert.Equal(t, int64(5), value)
}

func TestSQLVariantInt_ReportsAbsentForNil(t *testing.T) {
	// This is what tells an ordinary column apart from an identity column.
	_, ok := sqlVariantInt(nil)

	assert.False(t, ok)
}

// Host and port formatting.

func TestHostPort_JoinsAHostname(t *testing.T) {
	assert.Equal(t, "sqlserver.company.com:1433", hostPort("sqlserver.company.com", 1433))
}

func TestHostPort_BracketsAnIPv6Literal(t *testing.T) {
	assert.Equal(t, "[::1]:1433", hostPort("::1", 1433))
}

func TestHostPort_LeavesAnAlreadyBracketedHostAlone(t *testing.T) {
	assert.Equal(t, "[::1]:1433", hostPort("[::1]", 1433))
}

// Exclusion lists.

func TestLowercaseSet_MatchesRegardlessOfCase(t *testing.T) {
	// SQL Server names are case insensitive under the default collation, so
	// exclude_schemas: ["INFORMATION_SCHEMA"] has to match "information_schema".
	set := lowercaseSet([]string{"INFORMATION_SCHEMA", "sys"})

	assert.True(t, set["information_schema"])
	assert.True(t, set["sys"])
	assert.False(t, set["dbo"])
}

// Metadata hygiene.

func TestSetIfNotEmpty_SetsANonEmptyValue(t *testing.T) {
	metadata := map[string]any{}

	setIfNotEmpty(metadata, "collation", "SQL_Latin1_General_CP1_CI_AS")

	assert.Equal(t, "SQL_Latin1_General_CP1_CI_AS", metadata["collation"])
}

func TestSetIfNotEmpty_LeavesAnEmptyValueOut(t *testing.T) {
	// An empty key would show as a blank field in the catalog.
	metadata := map[string]any{}

	setIfNotEmpty(metadata, "comment", "")

	assert.NotContains(t, metadata, "comment")
}
