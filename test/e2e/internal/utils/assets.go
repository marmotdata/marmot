package utils

import (
	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
)

// FindAssetByName finds an asset by its name in a list of assets.
func FindAssetByName(assets []*models.AssetAsset, name string) *models.AssetAsset {
	for _, asset := range assets {
		if asset.Name == name {
			return asset
		}
	}
	return nil
}
