package mssql_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run against a real SQL Server. Start one with:
//
//	docker run -d --name marmot-test-mssql --memory 2500m -p 11433:1433 \
//	  -e ACCEPT_EULA=1 -e MSSQL_SA_PASSWORD='Marmot_Str0ng!' \
//	  mcr.microsoft.com/azure-sql-edge:latest
//
// then set MARMOT_TEST_MSSQL_HOST=localhost, MARMOT_TEST_MSSQL_PORT=11433,
// MARMOT_TEST_MSSQL_USER=sa and MARMOT_TEST_MSSQL_PASSWORD='Marmot_Str0ng!'.

func testConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_MSSQL_HOST")
	if host == "" {
		t.Skip("set MARMOT_TEST_MSSQL_HOST to run the SQL Server e2e tests")
	}

	port := 1433
	if raw := os.Getenv("MARMOT_TEST_MSSQL_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err, "MARMOT_TEST_MSSQL_PORT must be a number")
		port = parsed
	}

	return pluginsdk.RawConfig{
		"host":     host,
		"port":     port,
		"user":     os.Getenv("MARMOT_TEST_MSSQL_USER"),
		"password": os.Getenv("MARMOT_TEST_MSSQL_PASSWORD"),
		// The test image presents a self-signed certificate, so the
		// connection is encrypted but the certificate is not verified.
		"encrypt":                  true,
		"trust_server_certificate": true,
		"exclude_databases":        []any{"master", "model", "msdb", "tempdb"},
	}
}

// seed creates the shop and analytics databases the assertions below expect.
// It is idempotent, so re-running the suite against the same container works.
func seed(t *testing.T) {
	t.Helper()

	// Skips the test when no server is configured, before anything dials.
	testConfig(t)

	master := open(t, "master")
	defer master.Close()

	exec(t, master, `IF DB_ID('shop') IS NULL CREATE DATABASE shop`)
	exec(t, master, `IF DB_ID('analytics') IS NULL CREATE DATABASE analytics`)

	shop := open(t, "shop")
	defer shop.Close()

	exec(t, shop, `IF SCHEMA_ID('sales') IS NULL EXEC('CREATE SCHEMA sales')`)
	exec(t, shop, `IF OBJECT_ID('dbo.customers') IS NULL CREATE TABLE dbo.customers (
		customer_id INT IDENTITY(1,1) NOT NULL PRIMARY KEY,
		first_name NVARCHAR(100) NOT NULL,
		last_name NVARCHAR(100) NOT NULL,
		full_name AS (first_name + ' ' + last_name) PERSISTED,
		email VARCHAR(255) NULL,
		notes NVARCHAR(MAX) NULL,
		credit_limit DECIMAL(10,2) NULL DEFAULT ((0.00)),
		created_at DATETIME2(7) NOT NULL DEFAULT (SYSUTCDATETIME()))`)
	exec(t, shop, `IF OBJECT_ID('dbo.orders') IS NULL CREATE TABLE dbo.orders (
		order_id INT IDENTITY(1,1) NOT NULL PRIMARY KEY,
		customer_id INT NOT NULL,
		order_ref UNIQUEIDENTIFIER NOT NULL DEFAULT (NEWID()),
		total DECIMAL(10,2) NOT NULL,
		placed_at DATETIME2(3) NOT NULL DEFAULT (SYSUTCDATETIME()),
		CONSTRAINT FK_orders_customers FOREIGN KEY (customer_id)
			REFERENCES dbo.customers(customer_id) ON DELETE CASCADE ON UPDATE NO ACTION)`)
	exec(t, shop, `IF OBJECT_ID('sales.regions') IS NULL CREATE TABLE sales.regions (
		region_code CHAR(3) NOT NULL PRIMARY KEY,
		region_name VARCHAR(50) NOT NULL)`)
	exec(t, shop, `IF OBJECT_ID('dbo.order_totals') IS NULL EXEC('CREATE VIEW dbo.order_totals AS
		SELECT c.customer_id, c.full_name, COUNT(o.order_id) AS order_count, SUM(o.total) AS total_spend
		FROM dbo.customers c JOIN dbo.orders o ON o.customer_id = c.customer_id
		GROUP BY c.customer_id, c.full_name')`)
	exec(t, shop, `IF OBJECT_ID('sales.regional_orders') IS NULL EXEC('CREATE VIEW sales.regional_orders AS
		SELECT r.region_code, r.region_name FROM sales.regions AS r')`)
	exec(t, shop, `IF OBJECT_ID('dbo.get_customer') IS NULL EXEC('CREATE PROCEDURE dbo.get_customer @id INT AS
		BEGIN SELECT * FROM dbo.customers WHERE customer_id = @id END')`)
	exec(t, shop, `IF OBJECT_ID('dbo.customer_spend') IS NULL EXEC('CREATE FUNCTION dbo.customer_spend(@id INT) RETURNS DECIMAL(10,2) AS
		BEGIN DECLARE @t DECIMAL(10,2); SELECT @t = SUM(total) FROM dbo.orders WHERE customer_id = @id; RETURN ISNULL(@t, 0); END')`)
	exec(t, shop, `IF OBJECT_ID('dbo.orders_for_customer') IS NULL EXEC('CREATE FUNCTION dbo.orders_for_customer(@id INT) RETURNS TABLE AS
		RETURN (SELECT * FROM dbo.orders WHERE customer_id = @id)')`)

	// Extended properties are how SQL Server carries comments.
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM sys.extended_properties
		WHERE major_id = OBJECT_ID('dbo.customers') AND minor_id = 0 AND name = 'MS_Description')
		EXEC sp_addextendedproperty N'MS_Description', N'People who buy things', N'SCHEMA', N'dbo', N'TABLE', N'customers'`)
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM sys.extended_properties
		WHERE major_id = OBJECT_ID('dbo.customers')
		  AND minor_id = COLUMNPROPERTY(OBJECT_ID('dbo.customers'), 'email', 'ColumnId') AND name = 'MS_Description')
		EXEC sp_addextendedproperty N'MS_Description', N'Contact email address', N'SCHEMA', N'dbo', N'TABLE', N'customers', N'COLUMN', N'email'`)
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM sys.extended_properties
		WHERE major_id = OBJECT_ID('dbo.customers')
		  AND minor_id = COLUMNPROPERTY(OBJECT_ID('dbo.customers'), 'credit_limit', 'ColumnId') AND name = 'MS_Description')
		EXEC sp_addextendedproperty N'MS_Description', N'Maximum outstanding balance', N'SCHEMA', N'dbo', N'TABLE', N'customers', N'COLUMN', N'credit_limit'`)
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM sys.extended_properties
		WHERE class = 3 AND major_id = SCHEMA_ID('sales') AND name = 'MS_Description')
		EXEC sp_addextendedproperty N'MS_Description', N'Sales reference data', N'SCHEMA', N'sales'`)

	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM dbo.customers)
		INSERT INTO dbo.customers (first_name, last_name, email, credit_limit)
		VALUES (N'Ada', N'Lovelace', 'ada@example.com', 500.00), (N'Alan', N'Turing', 'alan@example.com', 250.50)`)
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM dbo.orders)
		INSERT INTO dbo.orders (customer_id, total) VALUES (1, 42.50), (1, 10.00), (2, 99.99)`)
	exec(t, shop, `IF NOT EXISTS (SELECT 1 FROM sales.regions)
		INSERT INTO sales.regions (region_code, region_name) VALUES ('EUW', 'Europe West'), ('USE', 'US East')`)

	analytics := open(t, "analytics")
	defer analytics.Close()

	exec(t, analytics, `IF OBJECT_ID('dbo.daily_totals') IS NULL CREATE TABLE dbo.daily_totals (
		day DATE NOT NULL PRIMARY KEY, revenue DECIMAL(18,4) NOT NULL)`)
	exec(t, analytics, `IF NOT EXISTS (SELECT 1 FROM dbo.daily_totals)
		INSERT INTO dbo.daily_totals (day, revenue) VALUES ('2026-01-01', 1200.5000)`)
}

func open(t *testing.T, database string) *sql.DB {
	t.Helper()

	port := os.Getenv("MARMOT_TEST_MSSQL_PORT")
	if port == "" {
		port = "1433"
	}

	query := url.Values{}
	query.Set("database", database)
	query.Set("encrypt", "true")
	query.Set("trustservercertificate", "true")
	query.Set("connection timeout", "30")

	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(os.Getenv("MARMOT_TEST_MSSQL_USER"), os.Getenv("MARMOT_TEST_MSSQL_PASSWORD")),
		Host:     os.Getenv("MARMOT_TEST_MSSQL_HOST") + ":" + port,
		RawQuery: query.Encode(),
	}

	db, err := sql.Open("sqlserver", u.String())
	require.NoError(t, err)
	require.NoError(t, db.Ping(), "cannot reach the test SQL Server")
	return db
}

func exec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	_, err := db.Exec(statement)
	require.NoErrorf(t, err, "seeding failed: %.80s", statement)
}

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	seed(t)

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := binary.Discover(ctx, testConfig(t))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// Meta and Validate over the wire.

func TestE2E_MetaOverTheWire(t *testing.T) {
	testConfig(t)

	binary := plugintest.Build(t, "..")
	meta, err := binary.Meta(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "mssql", meta.ID)
	assert.Equal(t, "Microsoft SQL Server", meta.Name)
	assert.Equal(t, "sql-server", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.True(t, meta.SupportsDataPreview, "the plugin implements DataFetcher")
}

func TestE2E_ValidateAcceptsAGoodConfig(t *testing.T) {
	binary := plugintest.Build(t, "..")

	_, err := binary.Validate(context.Background(), testConfig(t))

	assert.NoError(t, err)
}

func TestE2E_ValidateRejectsAMissingHost(t *testing.T) {
	config := testConfig(t)
	delete(config, "host")

	binary := plugintest.Build(t, "..")
	_, err := binary.Validate(context.Background(), config)

	assert.Error(t, err)
}

func TestE2E_ValidateRejectsAMissingPassword(t *testing.T) {
	config := testConfig(t)
	delete(config, "password")

	binary := plugintest.Build(t, "..")
	_, err := binary.Validate(context.Background(), config)

	assert.Error(t, err)
}

func TestE2E_DiscoverFailsAgainstAnUnreachableServer(t *testing.T) {
	config := testConfig(t)
	config["port"] = 1

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err := binary.Discover(ctx, config)

	assert.Error(t, err)
}

// Assets.

func TestE2E_DiscoversBothDatabasesAsContainers(t *testing.T) {
	result := discover(t)

	shop := findAsset(t, result, "Database", "shop")
	analytics := findAsset(t, result, "Database", "analytics")

	assert.Equal(t, "mrn://database/sql-server/shop", *shop.MRN)
	assert.Equal(t, "mrn://database/sql-server/analytics", *analytics.MRN)
	assert.Equal(t, []string{"SQL Server"}, shop.Providers)
}

func TestE2E_SkipsTheSystemDatabases(t *testing.T) {
	result := discover(t)

	for _, name := range []string{"master", "model", "msdb", "tempdb"} {
		assert.Nil(t, lookupAsset(result, "Database", name), "%s should be excluded", name)
	}
}

func TestE2E_DatabaseAssetReportsTheServerAndItsContents(t *testing.T) {
	result := discover(t)

	shop := findAsset(t, result, "Database", "shop")

	assert.Equal(t, "SQL_Latin1_General_CP1_CI_AS", shop.Metadata["collation"])
	assert.Equal(t, "ONLINE", shop.Metadata["state"])
	assert.Equal(t, "SIMPLE", shop.Metadata["recovery_model"])
	assert.NotEmpty(t, shop.Metadata["server_version"])
	assert.NotEmpty(t, shop.Metadata["edition"])
	assert.EqualValues(t, 3, shop.Metadata["table_count"])
	assert.EqualValues(t, 2, shop.Metadata["view_count"])
	// dbo and sales survive the schema exclusions.
	assert.EqualValues(t, 2, shop.Metadata["schema_count"])
}

func TestE2E_TableNamesAreFullyQualified(t *testing.T) {
	result := discover(t)

	for _, name := range []string{"shop.dbo.customers", "shop.dbo.orders", "shop.sales.regions"} {
		asset := findAsset(t, result, "Table", name)
		assert.Equal(t, "mrn://table/sql-server/"+name, *asset.MRN)
	}
}

func TestE2E_TheSameTableNameInTwoDatabasesStaysTwoAssets(t *testing.T) {
	// This is why the name is fully qualified: analytics.dbo.daily_totals and
	// shop.dbo.customers both live under dbo, and a bare name would collide.
	result := discover(t)

	assert.NotNil(t, lookupAsset(result, "Table", "analytics.dbo.daily_totals"))
	assert.NotNil(t, lookupAsset(result, "Table", "shop.dbo.customers"))
}

func TestE2E_TableCarriesItsExtendedPropertyAsADescription(t *testing.T) {
	result := discover(t)

	customers := findAsset(t, result, "Table", "shop.dbo.customers")

	require.NotNil(t, customers.Description)
	assert.Equal(t, "People who buy things", *customers.Description)
	assert.Equal(t, "People who buy things", customers.Metadata["comment"])
}

func TestE2E_TableInACommentedSchemaCarriesTheSchemaComment(t *testing.T) {
	result := discover(t)

	regions := findAsset(t, result, "Table", "shop.sales.regions")

	assert.Equal(t, "Sales reference data", regions.Metadata["schema_comment"])
	assert.Equal(t, "sales", regions.Metadata["schema"])
}

func TestE2E_SkipsTheSystemSchemas(t *testing.T) {
	result := discover(t)

	for _, asset := range result.Assets {
		schemaName, ok := asset.Metadata["schema"].(string)
		if !ok {
			continue
		}
		assert.NotEqual(t, "sys", schemaName)
		assert.NotEqual(t, "INFORMATION_SCHEMA", schemaName)
	}
}

// Columns.

func TestE2E_ColumnsRenderTheirTypesTheWaySQLServerDeclaresThem(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, "int", columns["customer_id"]["data_type"])
	assert.Equal(t, "nvarchar(100)", columns["first_name"]["data_type"], "nvarchar reports twice its character length")
	assert.Equal(t, "nvarchar(max)", columns["notes"]["data_type"])
	assert.Equal(t, "varchar(255)", columns["email"]["data_type"])
	assert.Equal(t, "decimal(10,2)", columns["credit_limit"]["data_type"])
	assert.Equal(t, "datetime2(7)", columns["created_at"]["data_type"])
}

func TestE2E_IdentityColumnIsMarkedWithItsSeedAndIncrement(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, true, columns["customer_id"]["is_identity"])
	assert.EqualValues(t, 1, columns["customer_id"]["identity_seed"])
	assert.EqualValues(t, 1, columns["customer_id"]["identity_increment"])
}

func TestE2E_ComputedColumnCarriesItsExpression(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, true, columns["full_name"]["is_computed"])
	assert.Equal(t, true, columns["full_name"]["is_persisted"])
	assert.Equal(t, "(([first_name]+' ')+[last_name])", columns["full_name"]["computed_definition"])
}

func TestE2E_PrimaryKeyColumnIsMarked(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, true, columns["customer_id"]["is_primary_key"])
	assert.Nil(t, columns["email"]["is_primary_key"], "a non-key column omits the flag")
}

func TestE2E_ColumnExtendedPropertiesBecomeDescriptions(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, "Contact email address", columns["email"]["description"])
	assert.Equal(t, "Maximum outstanding balance", columns["credit_limit"]["description"])
}

func TestE2E_ColumnNullabilityIsAlwaysReported(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, false, columns["first_name"]["is_nullable"])
	assert.Equal(t, true, columns["email"]["is_nullable"])
}

func TestE2E_ColumnDefaultConstraintIsReported(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "Table", "shop.dbo.customers"))

	assert.Equal(t, "((0.00))", columns["credit_limit"]["default_expression"])
}

func TestE2E_ViewsHaveColumnsToo(t *testing.T) {
	result := discover(t)

	columns := columnsOf(t, findAsset(t, result, "View", "shop.dbo.order_totals"))

	assert.Contains(t, columns, "customer_id")
	assert.Contains(t, columns, "total_spend")
}

// Views and routines.

func TestE2E_ViewCarriesItsDefinitionAsAQuery(t *testing.T) {
	result := discover(t)

	view := findAsset(t, result, "View", "shop.dbo.order_totals")

	require.NotNil(t, view.Query)
	require.NotNil(t, view.QueryLanguage)
	assert.Contains(t, *view.Query, "CREATE VIEW dbo.order_totals")
	assert.Equal(t, "SQL", *view.QueryLanguage)
}

func TestE2E_DiscoversStoredProceduresAndFunctionsAsFunctions(t *testing.T) {
	result := discover(t)

	procedure := findAsset(t, result, "Function", "shop.dbo.get_customer")
	scalar := findAsset(t, result, "Function", "shop.dbo.customer_spend")
	inline := findAsset(t, result, "Function", "shop.dbo.orders_for_customer")

	assert.Equal(t, "stored_procedure", procedure.Metadata["object_type"])
	assert.Equal(t, "scalar_function", scalar.Metadata["object_type"])
	assert.Equal(t, "inline_table_function", inline.Metadata["object_type"])
	assert.Equal(t, "mrn://function/sql-server/shop.dbo.get_customer", *procedure.MRN)
}

func TestE2E_FunctionCarriesItsBodyAsAQuery(t *testing.T) {
	result := discover(t)

	procedure := findAsset(t, result, "Function", "shop.dbo.get_customer")

	require.NotNil(t, procedure.Query)
	assert.Contains(t, *procedure.Query, "CREATE PROCEDURE dbo.get_customer")
	assert.Equal(t, false, procedure.Metadata["encrypted"])
}

// Lineage.

func TestE2E_DatabaseContainsItsTables(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "CONTAINS",
		"mrn://database/sql-server/shop", "mrn://table/sql-server/shop.dbo.orders"))
	assert.True(t, hasEdge(result, "CONTAINS",
		"mrn://database/sql-server/analytics", "mrn://table/sql-server/analytics.dbo.daily_totals"))
}

func TestE2E_DatabaseContainsItsViewsAndFunctions(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "CONTAINS",
		"mrn://database/sql-server/shop", "mrn://view/sql-server/shop.dbo.order_totals"))
	assert.True(t, hasEdge(result, "CONTAINS",
		"mrn://database/sql-server/shop", "mrn://function/sql-server/shop.dbo.get_customer"))
}

func TestE2E_ForeignKeyEdgeRunsFromTheReferencingTable(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "FOREIGN_KEY",
		"mrn://table/sql-server/shop.dbo.orders",
		"mrn://table/sql-server/shop.dbo.customers"))
}

func TestE2E_ViewOfEdgesRunFromTheBaseTablesToTheView(t *testing.T) {
	result := discover(t)

	assert.True(t, hasEdge(result, "VIEW_OF",
		"mrn://table/sql-server/shop.dbo.customers",
		"mrn://view/sql-server/shop.dbo.order_totals"))
	assert.True(t, hasEdge(result, "VIEW_OF",
		"mrn://table/sql-server/shop.dbo.orders",
		"mrn://view/sql-server/shop.dbo.order_totals"))
}

func TestE2E_ViewOfResolvesAnAliasedReferenceInAnotherSchema(t *testing.T) {
	// sales.regional_orders reads "FROM sales.regions AS r", so the alias must
	// not be mistaken for the object.
	result := discover(t)

	assert.True(t, hasEdge(result, "VIEW_OF",
		"mrn://table/sql-server/shop.sales.regions",
		"mrn://view/sql-server/shop.sales.regional_orders"))
}

func TestE2E_EveryEdgeEndpointIsAnAssetThisRunProduced(t *testing.T) {
	// The server silently drops an edge whose endpoint does not exist, so a
	// dangling edge would be invisible in production.
	result := discover(t)

	known := make(map[string]bool, len(result.Assets))
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, known[edge.Source], "edge source %s has no asset", edge.Source)
		assert.True(t, known[edge.Target], "edge target %s has no asset", edge.Target)
	}
}

// Statistics.

func TestE2E_RowCountsMatchTheSeededData(t *testing.T) {
	result := discover(t)

	assert.EqualValues(t, 2, statistic(t, result, "mrn://table/sql-server/shop.dbo.customers", "asset.row_count"))
	assert.EqualValues(t, 3, statistic(t, result, "mrn://table/sql-server/shop.dbo.orders", "asset.row_count"))
	assert.EqualValues(t, 2, statistic(t, result, "mrn://table/sql-server/shop.sales.regions", "asset.row_count"))
}

func TestE2E_ColumnCountsMatchTheDiscoveredColumns(t *testing.T) {
	result := discover(t)

	assert.EqualValues(t, 8, statistic(t, result, "mrn://table/sql-server/shop.dbo.customers", "asset.column_count"))
	assert.EqualValues(t, 5, statistic(t, result, "mrn://table/sql-server/shop.dbo.orders", "asset.column_count"))
}

func TestE2E_TableSizesAreReported(t *testing.T) {
	result := discover(t)

	assert.Greater(t, statistic(t, result, "mrn://table/sql-server/shop.dbo.orders", "asset.size_bytes"), float64(0))
}

func TestE2E_ViewsGetNoRowCount(t *testing.T) {
	// A view has no allocation metadata, and counting its rows would run the
	// underlying query.
	result := discover(t)

	for _, stat := range result.Statistics {
		if stat.MetricName == "asset.row_count" {
			assert.NotEqual(t, "mrn://view/sql-server/shop.dbo.order_totals", stat.AssetMRN)
		}
	}
}

// Configuration behaviour against the live server.

func TestE2E_DiscoveringOneDatabaseSkipsTheOthers(t *testing.T) {
	seed(t)

	config := testConfig(t)
	config["database"] = "analytics"

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := binary.Discover(ctx, config)
	require.NoError(t, err)

	assert.NotNil(t, lookupAsset(result, "Database", "analytics"))
	assert.Nil(t, lookupAsset(result, "Database", "shop"))
	assert.Nil(t, lookupAsset(result, "Table", "shop.dbo.orders"))
}

func TestE2E_TurningViewsOffDropsViewAssetsAndEdges(t *testing.T) {
	seed(t)

	config := testConfig(t)
	config["include_views"] = false

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := binary.Discover(ctx, config)
	require.NoError(t, err)

	assert.Nil(t, lookupAsset(result, "View", "shop.dbo.order_totals"))
	assert.NotNil(t, lookupAsset(result, "Table", "shop.dbo.orders"))
	for _, edge := range result.Lineage {
		assert.NotEqual(t, "VIEW_OF", edge.Type)
	}
}

func TestE2E_TurningProceduresOffDropsFunctionAssets(t *testing.T) {
	seed(t)

	config := testConfig(t)
	config["include_procedures"] = false

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := binary.Discover(ctx, config)
	require.NoError(t, err)

	assert.Nil(t, lookupAsset(result, "Function", "shop.dbo.get_customer"))
}

func TestE2E_TurningStatisticsOffEmitsNone(t *testing.T) {
	seed(t)

	config := testConfig(t)
	config["include_statistics"] = false

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := binary.Discover(ctx, config)
	require.NoError(t, err)

	assert.Empty(t, result.Statistics)
}

// Sample data.

func TestE2E_FetchSampleDataReturnsTheSeededRows(t *testing.T) {
	result := discover(t)

	orders := findAsset(t, result, "Table", "shop.dbo.orders")

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	columns, rows, err := binary.FetchSampleData(ctx, testConfig(t), &orders)
	require.NoError(t, err)

	assert.Equal(t, []string{"order_id", "customer_id", "order_ref", "total", "placed_at"}, columns)
	assert.Len(t, rows, 3)
}

func TestE2E_FetchSampleDataConvertsSQLServerTypesToJSONFriendlyValues(t *testing.T) {
	result := discover(t)

	orders := findAsset(t, result, "Table", "shop.dbo.orders")

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, rows, err := binary.FetchSampleData(ctx, testConfig(t), &orders)
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	// order_ref is a uniqueidentifier and arrives as 16 raw bytes; total is a
	// decimal and arrives as text. Both have to come out readable.
	assert.Regexp(t, `^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`, fmt.Sprint(rows[0][2]))
	assert.Regexp(t, `^\d+\.\d+$`, fmt.Sprint(rows[0][3]))
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T`, fmt.Sprint(rows[0][4]))
}

func TestE2E_FetchSampleDataWorksForAView(t *testing.T) {
	result := discover(t)

	view := findAsset(t, result, "View", "shop.dbo.order_totals")

	binary := plugintest.Build(t, "..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	columns, rows, err := binary.FetchSampleData(ctx, testConfig(t), &view)
	require.NoError(t, err)

	assert.Contains(t, columns, "total_spend")
	assert.Len(t, rows, 2, "two customers have orders")
}

// Helpers.

func lookupAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	asset := lookupAsset(result, assetType, name)
	require.NotNilf(t, asset, "no %s named %s in %d assets", assetType, name, len(result.Assets))
	return *asset
}

// columnsOf reads an asset's serialised column list, keyed by column name.
func columnsOf(t *testing.T, asset pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.Truef(t, ok, "%s has no columns", *asset.Name)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &list))

	columns := make(map[string]map[string]any, len(list))
	for _, column := range list {
		name, _ := column["column_name"].(string)
		columns[name] = column
	}
	return columns
}

func hasEdge(result *pluginsdk.DiscoveryResult, edgeType, source, target string) bool {
	for _, edge := range result.Lineage {
		if edge.Type == edgeType && edge.Source == source && edge.Target == target {
			return true
		}
	}
	return false
}

func statistic(t *testing.T, result *pluginsdk.DiscoveryResult, assetMRN, metric string) float64 {
	t.Helper()

	for _, stat := range result.Statistics {
		if stat.AssetMRN == assetMRN && stat.MetricName == metric {
			return stat.Value
		}
	}
	t.Fatalf("no %s statistic for %s", metric, assetMRN)
	return 0
}
