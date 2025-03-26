package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQSIngestion(t *testing.T) {
	// Create test queues on localstack
	testQueues := []utils.TestQueue{
		{
			Name: "test-queue-1",
			Tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
		{
			Name: "test-queue-2",
			Tags: map[string]string{
				"Environment": "staging",
				"Team":        "orders",
			},
		},
	}
	require.NoError(t, utils.CreateTestQueues(context.Background(), testQueues))

	configContent := fmt.Sprintf(`
runs:
  - sqs:
      credentials:
        region: us-east-1
        endpoint: http://localstack-test:%s
        id: test
        secret: test
      tags:
        - sqs
        - test
      tags_to_metadata: true
      include_tags:
        - Environment
        - Team
`, env.LocalstackPort)

	err := env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	queue1 := utils.FindAssetByName(resp.Payload.Assets, "test-queue-1")
	require.NotNil(t, queue1, "asset test-queue-1 not found")

	assert.Equal(t, "Queue", queue1.Type)
	assert.Contains(t, queue1.Providers, "SQS")
	assert.Contains(t, queue1.Tags, "sqs")
	assert.Contains(t, queue1.Tags, "test")
	// TODO: fix these assertions
	// assert.Equal(t, "prod", queue1.Metadata["Environment"])
	// assert.Equal(t, "platform", queue1.Metadata["Team"])

	// Verify second queue
	queue2 := utils.FindAssetByName(resp.Payload.Assets, "test-queue-2")
	require.NotNil(t, queue2, "asset test-queue-2 not found")

	assert.Equal(t, "Queue", queue2.Type)
	assert.Contains(t, queue2.Providers, "SQS")
	assert.Contains(t, queue2.Tags, "sqs")
	assert.Contains(t, queue2.Tags, "test")
	// TODO: fix these assertions
	// assert.Equal(t, "staging", queue2.Metadata["Environment"])
	// assert.Equal(t, "orders", queue2.Metadata["Team"])

	// Tidy up
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(queue1.ID))
	assert.NoError(t, err, "failed to delete asset", queue1.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(queue2.ID))
	assert.NoError(t, err, "failed to delete asset", queue2.ID)
}
