package postgresql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An asset's MRN is its identity: two objects sharing one become a single
// asset, and the second silently overwrites the first.

func TestAssetMRN_KeepsTheDatabaseAndSchemaInTheIdentity(t *testing.T) {
	// Discover walks every non-template database on the server, so both
	// levels have to be in the name. Without the database,
	// app_db.public.users and analytics_db.public.users collapse into one.
	assert.NotEqual(t,
		assetMRN("Table", "app_db", "public", "users"),
		assetMRN("Table", "analytics_db", "public", "users"),
		"the same table name in two databases must stay apart")

	assert.NotEqual(t,
		assetMRN("Table", "app_db", "public", "users"),
		assetMRN("Table", "app_db", "staging", "users"),
		"the same table name in two schemas must stay apart")
}

func TestAssetMRN_IsStableWhenTheServerRebuildsIt(t *testing.T) {
	// The UI splits the MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so the MRN must survive that unchanged.
	assert.Equal(t, "mrn://table/postgresql/shop.public.orders",
		assetMRN("Table", "shop", "public", "orders"))
}
