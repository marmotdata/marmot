package mcp

import (
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestFormatAssetCard_ShowsUserDescription(t *testing.T) {
	a := &asset.Asset{
		Name:            strPtr("orders"),
		Type:            "Table",
		Description:     strPtr("Orders table synced from Postgres."),
		UserDescription: strPtr("customer_id is the Stripe id, not the internal one."),
	}

	out := FormatAssetCard(a, "")

	assert.Contains(t, out, "Orders table synced from Postgres.")
	assert.Contains(t, out, "**User Notes:**")
	assert.Contains(t, out, "customer_id is the Stripe id, not the internal one.")
}

func TestFormatAssetCard_OmitsNotesWhenNoUserDescription(t *testing.T) {
	a := &asset.Asset{
		Name:        strPtr("orders"),
		Type:        "Table",
		Description: strPtr("Orders table synced from Postgres."),
	}

	out := FormatAssetCard(a, "")

	assert.NotContains(t, out, "**User Notes:**")
}

func TestFormatAssetCard_OmitsNotesWhenUserDescriptionEmpty(t *testing.T) {
	a := &asset.Asset{
		Name:            strPtr("orders"),
		Type:            "Table",
		UserDescription: strPtr(""),
	}

	out := FormatAssetCard(a, "")

	assert.NotContains(t, out, "**User Notes:**")
}
