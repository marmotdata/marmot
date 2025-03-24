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

func TestSNSIngestion(t *testing.T) {
	// Create test topics on localstack
	testTopics := []utils.TestTopic{
		{
			Name: "test-topic-1",
			Tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
		{
			Name: "test-topic-2",
			Tags: map[string]string{
				"Environment": "staging",
				"Team":        "orders",
			},
		},
	}

	require.NoError(t, utils.CreateTestTopics(context.Background(), testTopics))

	configContent := fmt.Sprintf(`
runs:
 - sns:
     credentials:
       region: us-east-1
       endpoint: http://localstack-test:%s
       id: test
       secret: test
     tags:
       - sns
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

	topic1 := utils.FindAssetByName(resp.Payload.Assets, "test-topic-1")
	require.NotNil(t, topic1, "asset test-topic-1 not found")

	assert.Equal(t, "Topic", topic1.Type)
	assert.Contains(t, topic1.Providers, "SNS")
	assert.Contains(t, topic1.Tags, "sns")
	assert.Contains(t, topic1.Tags, "test")
	//TODO: fix these assertions
	// assert.Equal(t, "prod", topic1.Metadata["Environment"])
	// assert.Equal(t, "platform", topic1.Metadata["Team"])

	topic2 := utils.FindAssetByName(resp.Payload.Assets, "test-topic-2")
	require.NotNil(t, topic2, "asset test-topic-2 not found")

	assert.Equal(t, "Topic", topic2.Type)
	assert.Contains(t, topic2.Providers, "SNS")
	assert.Contains(t, topic2.Tags, "sns")
	assert.Contains(t, topic2.Tags, "test")
	//TODO: fix these assertions
	// assert.Equal(t, "staging", topic2.Metadata["Environment"])
	// assert.Equal(t, "orders", topic2.Metadata["Team"])

	// Tidy up
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(topic1.ID))
	assert.NoError(t, err, "failed to delete asset", topic1.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(topic2.ID))
	assert.NoError(t, err, "failed to delete asset", topic2.ID)
}
