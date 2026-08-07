package release

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func TestFindBinary_MatchesGoReleaserLayout(t *testing.T) {
	dist := t.TempDir()

	// GoReleaser lays binaries out under
	//   marmot-plugin-postgresql_0.1.0_<os>_<arch>[_v1]/marmot-plugin-postgresql
	// (the trailing _v1 appears for GOAMD64 variants).
	writeFile(t, filepath.Join(dist, "marmot-plugin-postgresql_0.1.0_linux_amd64_v1", "marmot-plugin-postgresql"), "")
	writeFile(t, filepath.Join(dist, "marmot-plugin-postgresql_0.1.0_linux_arm64", "marmot-plugin-postgresql"), "")
	writeFile(t, filepath.Join(dist, "marmot-plugin-postgresql_0.1.0_linux_amd64_v1", "marmot-plugin-postgresql.txt"), "")

	got, err := findBinary(dist, "postgresql", Platform{OS: "linux", Arch: "amd64"})
	require.NoError(t, err)
	assert.Contains(t, got, "linux_amd64")
	assert.Equal(t, "marmot-plugin-postgresql", filepath.Base(got))

	_, err = findBinary(dist, "postgresql", Platform{OS: "windows", Arch: "amd64"})
	assert.Error(t, err, "missing platform must surface an error")
}

func TestPluginName(t *testing.T) {
	ref, err := registry.ParseReference("ghcr.io/marmotdata/plugins/postgresql:0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "postgresql", pluginName(ref))

	ref, err = registry.ParseReference("ghcr.io/marmotdata/postgresql:0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "postgresql", pluginName(ref))
}
