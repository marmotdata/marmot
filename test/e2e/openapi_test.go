package e2e

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	containerSpecsPath = "/tmp/openapi-specs"
	containerOpenAPIFilename = "openapi.yaml"
	testSpecPath = "testdata/openapi.yaml"
)

func TestOpenAPIIngestion(t *testing.T) {
	specsDir, err := filepath.Abs(containerSpecsPath)
	require.NoError(t, err, "Failed to get absolute path")

	if err := os.RemoveAll(specsDir); err != nil && !os.IsNotExist(err) {
		require.NoError(t, err, "Failed to clean up existing specs directory")
	}
	require.NoError(t, os.MkdirAll(specsDir, 0777), "Failed to create specs directory")

	specBytes, err := os.ReadFile(testSpecPath)
    	require.NoError(t, err, "Failed to read spec file from testdata")

	openapiSpecPath := filepath.Join(specsDir, containerOpenAPIFilename)
	require.NoError(t, os.WriteFile(openapiSpecPath, specBytes, 0666))

	files, err := os.ReadDir(specsDir)
	require.NoError(t, err, "Error reading spec directory")
	for _, file := range files {
		fileInfo, err := os.Stat(filepath.Join(specsDir, file.Name()))
		require.NoError(t, err)
		t.Logf("Spec file found: %s (size: %d, mode: %s)",
			file.Name(), fileInfo.Size(), fileInfo.Mode().String())
	}

	t.Logf("Contents of %s:", specsDir)
	for _, file := range files {
		t.Logf("  - %s", file.Name())
	}

	testFile := filepath.Join(specsDir, "test-file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0666))

	configContent := fmt.Sprintf(`
runs:
  - openapi:
      spec_path: "%s"
      tags:
        - "openapi"
        - "api"
`, containerSpecsPath)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey,
			"-H", "http://marmot-test:8080"},
		configContent,
		nil,
		fmt.Sprintf("%s:%s", specsDir, containerSpecsPath),
	)
	require.NoError(t, err)

	debugCmd := []string{"ls", "-la", containerSpecsPath}
	containerConfig := &container.Config{
		Image: "marmot:test",
		Cmd:   debugCmd,
	}
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(env.Config.NetworkName),
		Binds:       []string{fmt.Sprintf("%s:%s", specsDir, containerSpecsPath)},
	}
	debugContainerID, err := env.ContainerManager.StartContainer(containerConfig, hostConfig, "")
	require.NoError(t, err)
	defer env.ContainerManager.CleanupContainer(debugContainerID)

	debugOutput, err := env.ContainerManager.ExecCommand(debugContainerID, []string{"cat", path.Join(containerSpecsPath, containerOpenAPIFilename)})
	t.Logf("Debug container output: %s", debugOutput)

	t.Log("Ingest command executed, waiting for assets...")

	var resp *assets.GetAssetsListOK
	found := false

	for i := range 10 {
		time.Sleep(3 * time.Second)

		params := assets.NewGetAssetsListParams()
		resp, err = env.APIClient.Assets.GetAssetsList(params)
		require.NoError(t, err)

		if len(resp.Payload.Assets) > 0 {
			found = true
			break
		}

		t.Logf("No assets found yet (attempt %d/10)", i+1)
	}

	require.True(t, found, "No assets found after multiple attempts")

	openapiService := utils.FindAssetByName(resp.Payload.Assets, "Example.com")
	require.NotNil(t, openapiService, "OpenAPI service not found")
	assert.Equal(t, "Service", openapiService.Type)
	assert.Contains(t, openapiService.Tags, "openapi")
	assert.Contains(t, openapiService.Tags, "api")
	assert.Equal(t, len(openapiService.Tags), 2)

	t.Log("Cleaning up created assets...")
	assetIDs := []string{
		openapiService.ID,
	}

	for _, id := range assetIDs {
		_, err := env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(id))
		assert.NoError(t, err, "Failed to delete asset %s", id)
	}
}

