package cmd

import (
	"fmt"

	marmot "github.com/marmotdata/marmot/sdk/go"
)

// formatAssetName returns the display name for an asset.
func formatAssetName(a *marmot.Asset) string {
	if a.Name != "" {
		return a.Name
	}
	if a.Mrn != "" {
		return a.Mrn
	}
	return fmt.Sprintf("(%s)", a.ID)
}
