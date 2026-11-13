package hive

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSource(zone string) *Source {
	cfg := &Config{Host: "Hive.Internal", Port: 10000, IncludeColumns: true}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestDatabaseAsset_MRN(t *testing.T) {
	d := &discovery{source: testSource("")}

	a := d.databaseAsset("sales", &databaseInfo{Comment: "Sales data"})

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://asset/database/hive/sales", *a.MRN)
	assert.Equal(t, "sales", *a.Name)
}

func TestObjectAsset_MRNIsDatabaseThenTable(t *testing.T) {
	s := testSource("")
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := s.objectAsset("sales", "orders", info, nil, false, "")

	assert.Equal(t, "mrn://asset/table/hive/sales.orders", *a.MRN)
	assert.Equal(t, "orders", *a.Name, "the name is the table's own, the MRN carries the database")
	assert.Empty(t, a.PreviousMRNs, "the MRN is the one earlier releases produced")
}

func TestViewAsset_MRN(t *testing.T) {
	s := testSource("")
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	a := s.objectAsset("sales", "daily_totals", info, nil, false, "")

	assert.Equal(t, "mrn://asset/view/hive/sales.daily_totals", *a.MRN)
}

func TestObjectAsset_SameTableInTwoDatabasesDiffers(t *testing.T) {
	s := testSource("")
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := s.objectAsset("sales", "orders", info, nil, false, "")
	b := s.objectAsset("archive", "orders", info, nil, false, "")

	assert.NotEqual(t, *a.MRN, *b.MRN)
}

func TestObjectAsset_MRNCarriesTheZone(t *testing.T) {
	s := testSource("prod-eu")
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := s.objectAsset("sales", "orders", info, nil, false, "")
	d := (&discovery{source: s}).databaseAsset("sales", &databaseInfo{})

	assert.Equal(t, "mrn://asset/table/hive@prod-eu/sales.orders", *a.MRN)
	assert.Equal(t, "mrn://asset/database/hive@prod-eu/sales", *d.MRN)
}

func TestObjectAsset_MRNIsStableUnderRoundTrip(t *testing.T) {
	s := testSource("")
	info := parseDescribeFormatted(loadDescribe(t, "orders"))
	original := *s.objectAsset("sales", "orders", info, nil, false, "").MRN

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestMeta_Version(t *testing.T) {
	assert.Equal(t, ">=0.13.0", Meta().MarmotVersion)
}
