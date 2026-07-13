package gke

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookupConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"project_id": "my-project",
		"location":   "us-central1",
		"cluster":    "autopilot-cluster-1",
	}
}

func TestValidate_Valid(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(lookupConfig())
	require.NoError(t, err)
}

func TestValidate_RequiresProjectLocationCluster(t *testing.T) {
	for _, missing := range []string{"project_id", "location", "cluster"} {
		config := lookupConfig()
		delete(config, missing)
		s := &Source{}
		_, err := s.Validate(config)
		require.Error(t, err, "missing %s should fail", missing)
	}
}

func TestValidate_NothingToDiscover(t *testing.T) {
	config := lookupConfig()
	config["discover_namespaces"] = false
	config["discover_services"] = false
	config["discover_deployments"] = false
	config["discover_statefulsets"] = false
	config["discover_cronjobs"] = false
	config["discover_pods"] = false

	s := &Source{}
	_, err := s.Validate(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to discover")
}

func TestValidate_AppliesDiscoveryDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(lookupConfig())
	require.NoError(t, err)
	assert.True(t, s.config.DiscoverServices)
	assert.True(t, s.config.DiscoverCronJobs)
	assert.False(t, s.config.DiscoverPods)
}

func TestCredentials_InvalidJSON(t *testing.T) {
	config := &Config{}
	config.GCPConfig.Credentials.CredentialsJSON = "{not a service account key}"
	_, err := config.GCPConfig.TokenSource(context.Background(), gkeScope)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing GCP credentials")
}

func TestCredentials_MissingFile(t *testing.T) {
	config := &Config{}
	config.GCPConfig.Credentials.CredentialsFile = "/nonexistent/gcp-key.json"
	_, err := config.GCPConfig.TokenSource(context.Background(), gkeScope)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading GCP credentials file")
}
