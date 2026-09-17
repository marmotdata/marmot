package hive

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Hive table name is only unique within its database, so the asset Name
// is database.table and the MRN carries both.

func TestTableMRN_IsDatabaseDotTable(t *testing.T) {
	assert.Equal(t, "mrn://table/hive/sales.orders", assetMRN("Table", qualifiedName("sales", "orders")))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/hive/sales.daily_totals", assetMRN("View", qualifiedName("sales", "daily_totals")))
}

func TestDatabaseMRN_IsTheBareDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/hive/sales", assetMRN("Database", "sales"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical, dot
	// included.
	original := assetMRN("Table", qualifiedName("sales", "orders"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	d := &discovery{source: &Source{config: &Config{Host: "localhost", Port: 10000}}}

	a := d.databaseAsset("sales", &databaseInfo{Comment: "Sales data"})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://database/hive/sales", *a.MRN)
}

func TestObjectAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 10000, IncludeColumns: true}}
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := s.objectAsset("sales", "orders", info, nil, false, "")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://table/hive/sales.orders", *a.MRN)
	assert.Equal(t, "sales.orders", *a.Name, "the name people read carries the database")
}

func TestViewAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 10000, IncludeColumns: true}}
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	a := s.objectAsset("sales", "daily_totals", info, nil, false, "")

	require.NotNil(t, a.MRN)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://view/hive/sales.daily_totals", *a.MRN)
}

func TestTableMRN_MatchesWhatAnOpenMetadataImportOfAHiveServiceProduces(t *testing.T) {
	// plugins/openmetadata projects a Hive service as (Hive, schema.table)
	// grouped under a Database named after the Hive database, so both routes
	// land on one asset.
	assert.Equal(t, "mrn://table/hive/sales.orders", mrn.New("Table", "Hive", "sales.orders"))
	assert.Equal(t, "mrn://database/hive/sales", mrn.New("Database", "Hive", "sales"))
}

func TestTableMRN_UppercaseNamesAreLowercased(t *testing.T) {
	// Hive stores every identifier lowercased, and mrn.New lowercases too,
	// so a name typed in capitals cannot fork the asset.
	assert.Equal(t, assetMRN("Table", "sales.orders"), assetMRN("Table", "SALES.Orders"))
}
