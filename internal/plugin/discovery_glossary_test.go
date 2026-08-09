package plugin

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A plugin's result crosses the process boundary as JSON, so the host's
// mirror of the SDK types has to line up field for field. Anything that
// does not is dropped in silence.

func TestConvertDiscoveryResult_CarriesGlossaryTerms(t *testing.T) {
	result, err := convertDiscoveryResult(&pluginsdk.DiscoveryResult{
		GlossaryTerms: []pluginsdk.GlossaryTerm{{
			Name:        "BusinessTerms.Customer.LifetimeValue",
			Definition:  "Value over time.",
			Description: "What a customer is worth.",
			Parent:      "BusinessTerms.Customer",
			Synonyms:    []string{"CLV"},
			Tags:        []string{"finance"},
			Metadata:    map[string]interface{}{"glossary": "BusinessTerms"},
		}},
	})
	require.NoError(t, err)

	require.Len(t, result.GlossaryTerms, 1)
	term := result.GlossaryTerms[0]
	assert.Equal(t, "BusinessTerms.Customer.LifetimeValue", term.Name)
	assert.Equal(t, "Value over time.", term.Definition)
	assert.Equal(t, "What a customer is worth.", term.Description)
	assert.Equal(t, "BusinessTerms.Customer", term.Parent)
	assert.Equal(t, []string{"CLV"}, term.Synonyms)
	assert.Equal(t, []string{"finance"}, term.Tags)
	assert.Equal(t, "BusinessTerms", term.Metadata["glossary"])
}

func TestConvertDiscoveryResult_CarriesAnAssetsTermNames(t *testing.T) {
	name := "customers"
	result, err := convertDiscoveryResult(&pluginsdk.DiscoveryResult{
		Assets: []pluginsdk.Asset{{
			Name:      &name,
			Type:      "Table",
			Providers: []string{"PostgreSQL"},
			Terms:     []string{"BusinessTerms.Customer"},
		}},
	})
	require.NoError(t, err)

	require.Len(t, result.Assets, 1)
	assert.Equal(t, []string{"BusinessTerms.Customer"}, result.Assets[0].Terms)
}

func TestConvertDiscoveryResult_LeavesGlossaryEmptyWhenAPluginSendsNone(t *testing.T) {
	// Plugins built before terms existed send nothing, and must still
	// convert cleanly.
	result, err := convertDiscoveryResult(&pluginsdk.DiscoveryResult{})
	require.NoError(t, err)

	assert.Empty(t, result.GlossaryTerms)
}
