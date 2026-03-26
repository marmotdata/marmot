package cmd

import (
	"fmt"

	"github.com/marmotdata/marmot/client/models"
)

// formatAssetName returns the display name for a generated-client asset.
func formatAssetName(a *models.AssetAsset) string {
	if a.Name != "" {
		return a.Name
	}
	if a.Mrn != "" {
		return a.Mrn
	}
	return fmt.Sprintf("(%s)", a.ID)
}

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// int64Ptr returns a pointer to an int64 value.
func int64Ptr(v int) *int64 {
	i := int64(v)
	return &i
}

// strPtr returns a pointer to a string value.
func strPtr(v string) *string {
	return &v
}
