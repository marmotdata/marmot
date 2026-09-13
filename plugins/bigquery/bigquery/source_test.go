package bigquery

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProvider = "projects/123456789/locations/global/workloadIdentityPools/marmot/providers/marmot"

func TestValidateWithProviderFederates(t *testing.T) {
	raw := pluginsdk.RawConfig{
		"project_id":                 "acme-analytics",
		"workload_identity_provider": "//iam.googleapis.com/" + testProvider,
	}
	s := &Source{}
	out, err := s.Validate(raw)
	require.NoError(t, err)

	assert.Equal(t, "https://iam.googleapis.com/"+testProvider, pluginsdk.Audience(out), "the host learns the audience to mint for")
	assert.Equal(t, testProvider, s.config.WorkloadIdentityProvider, "the provider is kept bare")
	assert.True(t, s.config.IncludeDatasets, "defaults still apply")
}

func TestValidateWithKeyDoesNotFederate(t *testing.T) {
	for _, raw := range []pluginsdk.RawConfig{
		{"project_id": "p", "credentials_path": "/etc/marmot/key.json"},
		{"project_id": "p", "credentials_json": "{}"},
		{"project_id": "p", "use_default_credentials": true},
	} {
		out, err := (&Source{}).Validate(raw)
		require.NoError(t, err)
		assert.False(t, pluginsdk.Federated(out))
	}
}

func TestValidateRefusesProviderNextToAKey(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"project_id":                 "p",
		"workload_identity_provider": testProvider,
		"credentials_json":           "{}",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "excludes credentials_json")
}

func TestValidateRefusesServiceAccountWithoutProvider(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"project_id":       "p",
		"credentials_json": "{}",
		"service_account":  "sa@p.iam.gserviceaccount.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_account needs workload_identity_provider")
}

func TestValidateCountsProviderAsAnAuthMethod(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"project_id": "p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workload_identity_provider")

	_, err = (&Source{}).Validate(pluginsdk.RawConfig{
		"project_id":                 "p",
		"workload_identity_provider": testProvider,
		"use_default_credentials":    true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one authentication method")
}

func TestMetaSpecOffersFederation(t *testing.T) {
	names := map[string]bool{}
	for _, f := range Meta().ConfigSpec {
		names[f.Name] = true
	}
	for _, want := range []string{"workload_identity_provider", "service_account", "audience", "credentials_json", "project_id"} {
		assert.True(t, names[want], want)
	}
}
