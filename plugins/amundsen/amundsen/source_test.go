package amundsen

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"uri":      "bolt://neo4j.marmot.test:7687",
		"username": "neo4j",
		"password": "secret",
	}
}

func TestMeta_IdentifiesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "amundsen", meta.ID)
	assert.Equal(t, "Amundsen", meta.Name)
	assert.Equal(t, "catalog", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
}

func TestMeta_DeclaresOnlyWhatDiscoverEmits(t *testing.T) {
	assert.Equal(t, []string{"Assets", "Lineage"}, Meta().Features)
}

func TestMeta_ExposesAConfigForm(t *testing.T) {
	assert.NotEmpty(t, Meta().ConfigSpec)
}

func TestValidate_AcceptsTheMinimumConfig(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(validConfig())

	require.NoError(t, err)
}

func TestValidate_RequiresAURI(t *testing.T) {
	config := validConfig()
	delete(config, "uri")

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
}

func TestValidate_RequiresAUsername(t *testing.T) {
	config := validConfig()
	delete(config, "username")

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
}

func TestValidate_RequiresAPassword(t *testing.T) {
	config := validConfig()
	delete(config, "password")

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
}

func TestValidate_FillsInTheDefaults(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(validConfig())
	require.NoError(t, err)

	assert.Equal(t, "neo4j", source.config.Database)
	assert.True(t, source.config.IncludeUsers)
	assert.True(t, source.config.IncludeDashboards)
	assert.True(t, source.config.IncludeTags)
	assert.True(t, source.config.IncludeDescriptions)
	assert.True(t, source.config.IncludeUsage)
	assert.False(t, source.config.Encrypted)
	assert.False(t, source.config.TrustAllCertificates)
	assert.Equal(t, 120, source.config.QueryTimeoutSeconds)
	assert.Equal(t, 1000, source.config.PageSize)
}

func TestValidate_KeepsAnExplicitFalse(t *testing.T) {
	// A boolean that defaults to true still has to be switchable off.
	config := validConfig()
	config["include_dashboards"] = false

	source := &Source{}
	_, err := source.Validate(config)
	require.NoError(t, err)

	assert.False(t, source.config.IncludeDashboards)
}

func TestValidate_RejectsANonNeo4jScheme(t *testing.T) {
	config := validConfig()
	config["uri"] = "https://neo4j.marmot.test:7687"

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a Neo4j scheme")
}

func TestValidate_RejectsCredentialsInTheURI(t *testing.T) {
	// A password in the URI would end up in log lines and error text.
	config := validConfig()
	config["uri"] = "bolt://neo4j:secret@neo4j.marmot.test:7687"

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
}

func TestValidate_TrimsSpaceAroundTheURI(t *testing.T) {
	config := validConfig()
	config["uri"] = "  bolt://neo4j.marmot.test:7687 "

	source := &Source{}
	_, err := source.Validate(config)
	require.NoError(t, err)

	assert.Equal(t, "bolt://neo4j.marmot.test:7687", source.config.URI)
}

func TestValidate_TrimsATrailingSlashFromTheAmundsenURL(t *testing.T) {
	config := validConfig()
	config["amundsen_url"] = "https://amundsen.marmot.test/"

	source := &Source{}
	_, err := source.Validate(config)
	require.NoError(t, err)

	assert.Equal(t, "https://amundsen.marmot.test", source.config.AmundsenURL)
}

func TestValidate_RejectsAnAmundsenURLThatIsNotAURL(t *testing.T) {
	config := validConfig()
	config["amundsen_url"] = "amundsen"

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
}

func TestValidate_RejectsAPageSizeOfZero(t *testing.T) {
	// A zero page size would read the same first page forever.
	config := validConfig()
	config["page_size"] = 0

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
}

func TestBoltURI_LeavesAPlainAddressAlone(t *testing.T) {
	uri, err := boltURI("bolt://neo4j.marmot.test:7687", false, false)

	require.NoError(t, err)
	assert.Equal(t, "bolt://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_UpgradesToTLSWhenEncrypted(t *testing.T) {
	// The Neo4j Go driver takes encryption from the scheme rather than
	// from a setting, so the config has to rewrite the scheme.
	uri, err := boltURI("bolt://neo4j.marmot.test:7687", true, false)

	require.NoError(t, err)
	assert.Equal(t, "bolt+s://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_UpgradesToSelfSignedWhenTrustingAllCertificates(t *testing.T) {
	uri, err := boltURI("bolt://neo4j.marmot.test:7687", true, true)

	require.NoError(t, err)
	assert.Equal(t, "bolt+ssc://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_UpgradesARoutedAddressToo(t *testing.T) {
	uri, err := boltURI("neo4j://neo4j.marmot.test:7687", true, false)

	require.NoError(t, err)
	assert.Equal(t, "neo4j+s://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_LeavesAnAlreadyEncryptedSchemeAlone(t *testing.T) {
	uri, err := boltURI("bolt+ssc://neo4j.marmot.test:7687", false, false)

	require.NoError(t, err)
	assert.Equal(t, "bolt+ssc://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_TrustAllAloneDoesNotEncrypt(t *testing.T) {
	uri, err := boltURI("bolt://neo4j.marmot.test:7687", false, true)

	require.NoError(t, err)
	assert.Equal(t, "bolt://neo4j.marmot.test:7687", uri)
}

func TestBoltURI_RejectsAnAddressWithNoHost(t *testing.T) {
	_, err := boltURI("bolt://", false, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no host")
}
