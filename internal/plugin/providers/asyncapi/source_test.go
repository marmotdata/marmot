package asyncapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncAPIV3Discovery(t *testing.T) {
	// Get the testasyncapi directory path
	testDir := filepath.Join("..", "..", "..", "..", "testasyncapi")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testasyncapi directory not found, skipping test")
	}

	rawConfig := plugin.RawPluginConfig{
		"spec_path":         testDir,
		"discover_services": true,
		"discover_channels": true,
		"discover_messages": true,
	}

	source := &Source{}
	_, err := source.Validate(rawConfig)
	require.NoError(t, err)

	result, err := source.Discover(context.Background(), rawConfig)
	require.NoError(t, err)

	// Should have assets
	assert.NotEmpty(t, result.Assets, "Expected to discover assets")

	// Find service assets
	var services, topics, queues, exchanges []string
	for _, asset := range result.Assets {
		switch asset.Type {
		case "Service":
			services = append(services, *asset.Name)
		case "Topic":
			topics = append(topics, *asset.Name)
		case "Queue":
			queues = append(queues, *asset.Name)
		case "Exchange":
			exchanges = append(exchanges, *asset.Name)
		}
	}

	t.Logf("Services: %v", services)
	t.Logf("Topics: %v", topics)
	t.Logf("Queues: %v", queues)
	t.Logf("Exchanges: %v", exchanges)

	// Should have at least some services discovered
	assert.NotEmpty(t, services, "Expected to discover services")

	// Should have lineage edges
	assert.NotEmpty(t, result.Lineage, "Expected to have lineage edges")

	t.Logf("\n=== Lineage Edges (%d) ===", len(result.Lineage))
	for _, edge := range result.Lineage {
		t.Logf("  %s -[%s]-> %s", edge.Source, edge.Type, edge.Target)
	}
}

func TestExtractOperationChannelMappings(t *testing.T) {
	// Create a temp spec file
	spec := `asyncapi: 3.0.0
info:
  title: Test Service
  version: 1.0.0
channels:
  userCreated:
    address: user.created
  userDeleted:
    address: user.deleted
operations:
  publishUserCreated:
    action: send
    channel:
      $ref: '#/channels/userCreated'
  consumeUserDeleted:
    action: receive
    channel:
      $ref: '#/channels/userDeleted'
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(specPath, []byte(spec), 0644)
	require.NoError(t, err)

	source := &Source{}
	mappings := source.extractOperationChannelMappings(specPath)

	assert.Len(t, mappings, 2)
	assert.Equal(t, "userCreated", mappings["publishUserCreated"])
	assert.Equal(t, "userDeleted", mappings["consumeUserDeleted"])
}

func TestExtractChannelNameFromRef(t *testing.T) {
	tests := []struct {
		ref      string
		expected string
	}{
		{"#/channels/userCreated", "userCreated"},
		{"#/channels/order/created", "order/created"},
		{"#/servers/production", ""},
		{"invalid", ""},
		{"#/channels/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			result := extractChannelNameFromRef(tt.ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}
