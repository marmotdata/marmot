package cloudrun

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Cloud Run workload is addressed by region and id, because one id can exist
// in several regions of the same project. mrn.New lowercases every part and
// turns slashes and spaces into hyphens, so the provider loses its space and
// the name loses its slash.

func TestServiceMRN_IsTheRegionAndServiceID(t *testing.T) {
	assert.Equal(t, "mrn://service/cloud-run/europe-west1-checkout-api",
		assetMRN(typeService, "europe-west1/checkout-api"))
}

func TestJobMRN_IsTheRegionAndJobID(t *testing.T) {
	assert.Equal(t, "mrn://job/cloud-run/europe-west1-nightly-export",
		assetMRN(typeJob, "europe-west1/nightly-export"))
}

func TestServiceMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN(typeService, "europe-west1/checkout-api")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestJobMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN(typeJob, "europe-west1/nightly-export")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// Lineage points at an asset the gcs plugin owns, so its MRN is pinned here
// too: a drift in either plugin's naming silently drops the edge on the server.
func TestGCSBucketMRN_MatchesTheGCSPlugin(t *testing.T) {
	assert.Equal(t, "mrn://bucket/gcs/order-archive", mrn.New(gcsBucketType, gcsProvider, "order-archive"))
}

// The server rebuilds identity from (Type, Providers[0], Name), so every
// asset's MRN has to equal mrn.New over its own fields or the asset is stored
// under a different identity than the one lineage points at.
func TestEveryAssetMRN_EqualsMRNNewOverItsOwnFields(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.NotEmpty(t, asset.Providers)

		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN, "asset %s", *asset.Name)
	}
}

// Every lineage edge and every statistic has to address an asset by the same
// MRN the asset was stored under, or the server drops it.
func TestEveryEdgeAndStatistic_PointsAtAnAssetFromTheSameRun(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	known := map[string]bool{}
	for _, asset := range result.Assets {
		known[*asset.MRN] = true
	}

	for _, edge := range result.Lineage {
		assert.True(t, known[edge.Target], "edge target %s is not an asset from this run", edge.Target)
	}
	for _, statistic := range result.Statistics {
		assert.True(t, known[statistic.AssetMRN], "statistic %s is not an asset from this run", statistic.AssetMRN)
	}
	for _, history := range result.RunHistory {
		assert.True(t, known[history.AssetMRN], "run history %s is not an asset from this run", history.AssetMRN)
	}
}
