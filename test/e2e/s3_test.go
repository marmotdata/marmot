package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3Ingestion(t *testing.T) {
	testBuckets := []utils.TestBucket{
		{
			Name: "test-bucket-1",
			Tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
		{
			Name: "test-bucket-2",
			Tags: map[string]string{
				"Environment": "staging",
				"Team":        "analytics",
			},
		},
	}
	require.NoError(t, utils.CreateTestBuckets(context.Background(), testBuckets))

	configContent := fmt.Sprintf(`
runs:
  - s3:
      credentials:
        region: us-east-1
        endpoint: http://localstack-test:%s
        id: test
        secret: test
      tags:
        - s3
        - test
      tags_to_metadata: true
      include_tags:
        - Environment
        - Team
`, env.LocalstackPort)

	err := env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	bucket1 := utils.FindAssetByName(resp.Payload.Assets, "test-bucket-1")
	require.NotNil(t, bucket1, "asset test-bucket-1 not found")

	assert.Equal(t, "Bucket", bucket1.Type)
	assert.Contains(t, bucket1.Providers, "S3")
	assert.Contains(t, bucket1.Tags, "s3")
	assert.Contains(t, bucket1.Tags, "test")

	bucket2 := utils.FindAssetByName(resp.Payload.Assets, "test-bucket-2")
	require.NotNil(t, bucket2, "asset test-bucket-2 not found")

	assert.Equal(t, "Bucket", bucket2.Type)
	assert.Contains(t, bucket2.Providers, "S3")
	assert.Contains(t, bucket2.Tags, "s3")
	assert.Contains(t, bucket2.Tags, "test")

	for _, bucket := range []*models.AssetAsset{bucket1, bucket2} {
		assert.NotNil(t, bucket.Metadata, "bucket should have metadata")
		metadata := bucket.Metadata.(map[string]interface{})
		assert.NotEmpty(t, metadata, "metadata should not be empty")

		assert.Contains(t, metadata, "tag_Environment")
		assert.Contains(t, metadata, "tag_Team")
	}

	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(bucket1.ID))
	assert.NoError(t, err, "failed to delete asset", bucket1.ID)
	_, err = env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(bucket2.ID))
	assert.NoError(t, err, "failed to delete asset", bucket2.ID)
}
