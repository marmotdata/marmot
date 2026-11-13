package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discoverSpecs(t *testing.T, config pluginsdk.RawConfig, titles ...string) *pluginsdk.DiscoveryResult {
	t.Helper()

	dir := t.TempDir()
	for _, title := range titles {
		spec := []byte("openapi: 3.0.0\ninfo:\n  title: " + title + "\n  version: \"1.0\"\npaths:\n  /users:\n    get:\n      summary: List users\n      responses:\n        \"200\":\n          description: ok\n")
		require.NoError(t, os.WriteFile(filepath.Join(dir, title+".yaml"), spec, 0o600))
	}

	config["spec_path"] = dir
	result, err := (&Source{}).Discover(context.Background(), config)
	require.NoError(t, err)
	return result
}

func mrns(result *pluginsdk.DiscoveryResult) map[string]bool {
	out := make(map[string]bool)
	for _, a := range result.Assets {
		out[*a.MRN] = true
	}
	return out
}

func TestEndpointMRN_IsQualifiedByItsService(t *testing.T) {
	result := discoverSpecs(t, pluginsdk.RawConfig{}, "billing", "orders")

	got := mrns(result)
	assert.Len(t, got, 4)
	assert.True(t, got["mrn://asset/service/openapi/billing"])
	assert.True(t, got["mrn://asset/endpoint/openapi/billing.get--users"])
	assert.True(t, got["mrn://asset/endpoint/openapi/orders.get--users"])
}

func TestMRN_CarriesTheZone(t *testing.T) {
	result := discoverSpecs(t, pluginsdk.RawConfig{"zone": "prod"}, "billing")

	got := mrns(result)
	assert.True(t, got["mrn://asset/service/openapi@prod/billing"])
	assert.True(t, got["mrn://asset/endpoint/openapi@prod/billing.get--users"])
	for _, a := range result.Assets {
		if a.ParentMRN != nil {
			assert.Equal(t, "mrn://asset/service/openapi@prod/billing", *a.ParentMRN)
		}
	}
}
