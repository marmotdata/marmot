package sftp

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An asset is named by its path below the configured root, so two
// folders can each hold an orders.csv without colliding. mrn.New turns
// the slashes into hyphens.

// assetMRN builds an MRN without a zone.
func assetMRN(assetType, name string) string {
	return (&Config{}).assetMRN(assetType, name)
}

func TestMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "dropbox-eu"
	assert.Equal(t, "mrn://asset/file/sftp@dropbox-eu/incoming-orders.csv", cfg.assetMRN(typeFile, "incoming/orders.csv"))
}

func TestFolderMRN_IsThePathBelowTheRoot(t *testing.T) {
	assert.Equal(t, "mrn://asset/folder/sftp/incoming", assetMRN(typeFolder, "incoming"))
}

func TestFolderMRN_TurnsPathSeparatorsIntoHyphens(t *testing.T) {
	assert.Equal(t, "mrn://asset/folder/sftp/incoming-2026-09", assetMRN(typeFolder, "incoming/2026-09"))
}

func TestFileMRN_CarriesTheFolderItSitsIn(t *testing.T) {
	assert.Equal(t, "mrn://asset/file/sftp/incoming-2026-09-orders.csv", assetMRN(typeFile, "incoming/2026-09/orders.csv"))
}

func TestFileMRN_AtTheTopOfARootIsTheBareFileName(t *testing.T) {
	assert.Equal(t, "mrn://asset/file/sftp/manifest.csv", assetMRN(typeFile, "manifest.csv"))
}

func TestMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := assetMRN(typeFile, "incoming/2026-09/orders.csv")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, parsed.String())
}

func TestDiscoveredAssets_MRNMatchesTheirOwnFields(t *testing.T) {
	// The Marmot server rebuilds identity from (Type, Providers[0], Name),
	// so an asset whose MRN says anything else is unreachable.
	result := discoverTree(t, writeTree(t), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.Len(t, a.Providers, 1)

		assert.Equal(t, mrn.Asset(a.Type, a.Providers[0], *a.Name).String(), *a.MRN)
	}
}

func TestDiscoveredAssets_UseOnlyFolderAndFileTypes(t *testing.T) {
	result := discoverTree(t, writeTree(t), nil)

	for _, a := range result.Assets {
		assert.Contains(t, []string{typeFolder, typeFile}, a.Type)
	}
}

func TestCollect_BindsTheHostToTheZone(t *testing.T) {
	s := &Source{config: testConfig(t, writeTree(t), pluginsdk.RawConfig{"zone": "dropbox-eu", "host": "SFTP.Example.com", "port": 2222})}
	result := s.collect(context.Background(), osFS{})

	assert.Equal(t, []pluginsdk.ZoneBinding{{Provider: "SFTP", Host: "sftp.example.com:2222", Zone: "dropbox-eu"}}, result.ZoneBindings)
}
