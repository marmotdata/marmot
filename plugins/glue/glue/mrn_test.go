package glue

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/stretchr/testify/assert"
)

func newMRNSource(zone string) *Source {
	cfg := &Config{}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func tableNamed(db, name string) (string, types.Table) {
	return db, types.Table{Name: &name}
}

func TestTableMRN_IsDatabaseThenTable(t *testing.T) {
	s := newMRNSource("")
	asset := s.createTableAsset(tableNamed("shop", "orders"))

	assert.Equal(t, "mrn://asset/table/glue/shop.orders", *asset.MRN)
	assert.Equal(t, "orders", *asset.Name)
	assert.Empty(t, asset.PreviousMRNs, "a plain name keeps the MRN earlier releases gave it")
}

func TestTableMRN_SameNameInTwoDatabasesDiffers(t *testing.T) {
	s := newMRNSource("")
	a := s.createTableAsset(tableNamed("shop", "orders"))
	b := s.createTableAsset(tableNamed("archive", "orders"))

	assert.NotEqual(t, *a.MRN, *b.MRN)
}

func TestMRNs_CarryTheZone(t *testing.T) {
	s := newMRNSource("prod-eu")
	table := s.createTableAsset(tableNamed("shop", "orders"))
	db := s.createDatabaseAsset(types.Database{Name: strPtr("shop")})

	assert.Equal(t, "mrn://asset/table/glue@prod-eu/shop.orders", *table.MRN)
	assert.Equal(t, "mrn://asset/database/glue@prod-eu/shop", *db.MRN)
}

func TestPreviousTableMRNs_OnlyForDottedNames(t *testing.T) {
	assert.Nil(t, previousTableMRNs("shop", "orders"))
	assert.Equal(t, []string{"mrn://table/glue/my.db.orders"}, previousTableMRNs("my.db", "orders"))
}

func strPtr(s string) *string { return &s }
