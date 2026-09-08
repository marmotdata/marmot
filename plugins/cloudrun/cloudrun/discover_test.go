package cloudrun

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	run "google.golang.org/api/run/v2"
)

func TestDiscover_CreatesAServiceAsset(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	asset := assetNamed(t, result, "europe-west1/checkout-api")
	assert.Equal(t, "Service", asset.Type)
	assert.Equal(t, []string{"Cloud Run"}, asset.Providers)
	require.NotNil(t, asset.Description)
	assert.Equal(t, "Takes checkout requests from the storefront", *asset.Description)
}

func TestDiscover_QualifiesTheAssetNameByRegion(t *testing.T) {
	// One service id can exist in several regions of the same project, and
	// every region is scanned by default, so the region has to be part of the
	// name for the two to stay separate assets.
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{{
		Services: []*run.GoogleCloudRunV2Service{
			{Name: serviceName("europe-west1", "checkout-api")},
			{Name: serviceName("us-central1", "checkout-api")},
		},
	}}

	result := discover(t, fake, pluginsdk.RawConfig{"include_jobs": false})

	assetNamed(t, result, "europe-west1/checkout-api")
	assetNamed(t, result, "us-central1/checkout-api")
}

func TestDiscover_ScansEveryRegionWithTheWildcardParent(t *testing.T) {
	fake := newFakeAPI()

	discover(t, fake, nil)

	assert.Equal(t, []string{"projects/acme/locations/-"}, fake.parents)
}

func TestDiscover_ScansOnlyTheConfiguredRegions(t *testing.T) {
	fake := newFakeAPI()

	discover(t, fake, pluginsdk.RawConfig{"locations": []string{"europe-west1", "us-central1"}})

	assert.Equal(t, []string{
		"projects/acme/locations/europe-west1",
		"projects/acme/locations/us-central1",
	}, fake.parents)
}

func TestDiscover_RecordsServiceMetadata(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "3fdbba8e-6a51-4f18-9b0a-1b6b1f1a2c3d", metadata["uid"])
	assert.Equal(t, "europe-west1", metadata["location"])
	assert.Equal(t, "acme", metadata["project_id"])
	assert.Equal(t, "https://checkout-api-abcdef-ew.a.run.app", metadata["uri"])
	assert.Equal(t, "INGRESS_TRAFFIC_ALL", metadata["ingress"])
	assert.Equal(t, "checkout-api@acme.iam.gserviceaccount.com", metadata["service_account"])
	assert.Equal(t, "CONDITION_SUCCEEDED", metadata["ready"])
	assert.Equal(t, true, metadata["reconciling"])
	assert.Equal(t, int64(7), metadata["generation"])
	assert.Equal(t, int64(80), metadata["max_instance_request_concurrency"])
	assert.Equal(t, int64(1), metadata["min_instance_count"])
	assert.Equal(t, int64(25), metadata["max_instance_count"])
}

func TestDiscover_ShortensRevisionNamesToTheirBareID(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "checkout-api-00042-abc", metadata["latest_ready_revision"])
	assert.Equal(t, "checkout-api-00043-def", metadata["latest_created_revision"])
}

func TestDiscover_RecordsLabelsUnderAPrefix(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "payments", metadata["label_team"])
	assert.Equal(t, "prod", metadata["label_env"])
}

func TestDiscover_RecordsContainerImageAndPorts(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "europe-docker.pkg.dev/acme/services/checkout-api:2.11.0", metadata["container_image"])
	assert.Equal(t, []string{"europe-docker.pkg.dev/acme/services/checkout-api:2.11.0"}, metadata["container_images"])
	assert.Equal(t, []int64{8080}, metadata["container_ports"])
}

func TestDiscover_RecordsEnvVarNamesButNeverTheirValues(t *testing.T) {
	// Container environment routinely holds credentials, so a value must
	// never reach the catalog.
	result := discover(t, newFakeAPI(), nil)

	asset := assetNamed(t, result, "europe-west1/checkout-api")
	assert.Equal(t, []string{"DATABASE_PASSWORD", "LOG_LEVEL"}, asset.Metadata["env_var_names"])
	for key, value := range asset.Metadata {
		assert.NotEqual(t, "hunter2", value, "metadata key %s leaked an env var value", key)
	}
}

func TestDiscover_RecordsVpcAccess(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "projects/acme/locations/europe-west1/connectors/serverless-egress", metadata["vpc_connector"])
	assert.Equal(t, "PRIVATE_RANGES_ONLY", metadata["vpc_egress"])
}

func TestDiscover_RecordsVolumesByKind(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, []string{"order-archive"}, metadata["gcs_volume_buckets"])
	assert.Equal(t, []string{"acme:europe-west1:orders-db"}, metadata["cloud_sql_instances"])
	assert.Equal(t, []string{"checkout-signing-key"}, metadata["secret_volumes"])
	assert.Equal(t, []string{"10.0.0.4:/exports/shared"}, metadata["nfs_volumes"])
}

func TestDiscover_OmitsMetadataKeysTheAPIDidNotSet(t *testing.T) {
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{{
		Services: []*run.GoogleCloudRunV2Service{{Name: serviceName(testRegion, "bare")}},
	}}

	result := discover(t, fake, pluginsdk.RawConfig{"include_jobs": false})

	metadata := assetNamed(t, result, "europe-west1/bare").Metadata
	assert.NotContains(t, metadata, "uri")
	assert.NotContains(t, metadata, "generation")
	assert.NotContains(t, metadata, "reconciling")
	assert.NotContains(t, metadata, "gcs_volume_buckets")
}

func TestDiscover_SkipsAServiceWithAnUnparseableName(t *testing.T) {
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{{
		Services: []*run.GoogleCloudRunV2Service{
			{Name: "not-a-resource-name"},
			{Name: serviceName(testRegion, "checkout-api")},
		},
	}}

	result := discover(t, fake, pluginsdk.RawConfig{"include_jobs": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "europe-west1/checkout-api", *result.Assets[0].Name)
}

func TestDiscover_FollowsServicePageTokensToTheEnd(t *testing.T) {
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{
		{
			Services:      []*run.GoogleCloudRunV2Service{{Name: serviceName(testRegion, "page-one")}},
			NextPageToken: "second",
		},
		{
			Services: []*run.GoogleCloudRunV2Service{{Name: serviceName(testRegion, "page-two")}},
		},
	}

	result := discover(t, fake, pluginsdk.RawConfig{"include_jobs": false})

	assert.Equal(t, 2, fake.serviceCalls)
	assetNamed(t, result, "europe-west1/page-one")
	assetNamed(t, result, "europe-west1/page-two")
}

func TestDiscover_FollowsJobPageTokensToTheEnd(t *testing.T) {
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{{}}
	fake.jobPages = []*run.GoogleCloudRunV2ListJobsResponse{
		{
			Jobs:          []*run.GoogleCloudRunV2Job{{Name: jobName(testRegion, "job-one")}},
			NextPageToken: "second",
		},
		{
			Jobs: []*run.GoogleCloudRunV2Job{{Name: jobName(testRegion, "job-two")}},
		},
	}

	result := discover(t, fake, nil)

	assert.Equal(t, 2, fake.jobCalls)
	assetNamed(t, result, "europe-west1/job-one")
	assetNamed(t, result, "europe-west1/job-two")
}

func TestDiscover_KeepsGoingWhenALocationIsUnreachable(t *testing.T) {
	// Cloud Run reports a region it could not reach in the response body
	// rather than failing, so the run must keep the services it did get.
	fake := newFakeAPI()
	fake.servicePages = []*run.GoogleCloudRunV2ListServicesResponse{{
		Services:    []*run.GoogleCloudRunV2Service{{Name: serviceName(testRegion, "checkout-api")}},
		Unreachable: []string{"projects/acme/locations/asia-south2"},
	}}

	result := discover(t, fake, pluginsdk.RawConfig{"include_jobs": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "europe-west1/checkout-api", *result.Assets[0].Name)
}

func TestDiscover_WithoutJobsReturnsServicesOnly(t *testing.T) {
	result := discover(t, newFakeAPI(), pluginsdk.RawConfig{"include_jobs": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Service", result.Assets[0].Type)
	assert.Empty(t, result.Statistics)
	assert.Empty(t, result.RunHistory)
}

func TestDiscover_WithNoJobsInTheProjectStillReturnsServices(t *testing.T) {
	fake := newFakeAPI()
	fake.jobPages = []*run.GoogleCloudRunV2ListJobsResponse{{}}

	result := discover(t, fake, nil)

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Service", result.Assets[0].Type)
}

func TestDiscover_CreatesAJobAsset(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	asset := assetNamed(t, result, "europe-west1/nightly-export")
	assert.Equal(t, "Job", asset.Type)
	assert.Equal(t, []string{"Cloud Run"}, asset.Providers)
	assert.Nil(t, asset.Description, "Cloud Run jobs have no description field")
}

func TestDiscover_RecordsJobMetadataFromTheNestedTaskTemplate(t *testing.T) {
	// Job.Template is an execution template; the task template one level down
	// is what carries the containers, the volumes and the retry policy.
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/nightly-export").Metadata
	assert.Equal(t, int64(4), metadata["task_count"])
	assert.Equal(t, int64(2), metadata["parallelism"])
	assert.Equal(t, int64(3), metadata["max_retries"])
	assert.Equal(t, "3600s", metadata["timeout"])
	assert.Equal(t, "nightly-export@acme.iam.gserviceaccount.com", metadata["service_account"])
	assert.Equal(t, "europe-docker.pkg.dev/acme/jobs/nightly-export:1.4.2", metadata["container_image"])
	assert.Equal(t, []string{"order-archive"}, metadata["gcs_volume_buckets"])
	assert.Equal(t, []string{"EXPORT_TOKEN"}, metadata["env_var_names"])
	assert.Equal(t, "analytics", metadata["label_team"])
}

func TestDiscover_ShortensTheLatestExecutionToItsBareID(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/nightly-export").Metadata
	assert.Equal(t, "nightly-export-x9k2t", metadata["latest_created_execution"])
}

func TestDiscover_EmitsAnExecutionCountStatisticForEveryJob(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	require.Len(t, result.Statistics, 1)
	assert.Equal(t, "mrn://job/cloud run/europe-west1-nightly-export", result.Statistics[0].AssetMRN)
	assert.Equal(t, "asset.execution_count", result.Statistics[0].MetricName)
	assert.Equal(t, float64(3), result.Statistics[0].Value)
}

func TestDiscover_EmitsAnExecutionCountOfZero(t *testing.T) {
	// A job that has never run is still worth reporting a count for, so the
	// catalog shows zero rather than nothing.
	fake := newFakeAPI()
	fake.jobPages = []*run.GoogleCloudRunV2ListJobsResponse{{
		Jobs: []*run.GoogleCloudRunV2Job{{Name: jobName(testRegion, "never-run")}},
	}}

	result := discover(t, fake, nil)

	require.Len(t, result.Statistics, 1)
	assert.Equal(t, float64(0), result.Statistics[0].Value)
}

func TestDiscover_EmitsAFeedsEdgeFromEveryMountedBucket(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://bucket/gcs/order-archive",
		Target: "mrn://service/cloud run/europe-west1-checkout-api",
		Type:   "FEEDS",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://bucket/gcs/order-archive",
		Target: "mrn://job/cloud run/europe-west1-nightly-export",
		Type:   "FEEDS",
	})
}

func TestDiscover_EmitsOneEdgePerBucketEvenWhenMountedTwice(t *testing.T) {
	// checkoutService mounts order-archive under two volume names.
	result := discover(t, newFakeAPI(), pluginsdk.RawConfig{"include_jobs": false})

	assert.Len(t, result.Lineage, 1)
}

func TestDiscover_EmitsNoEdgeForACloudSQLVolume(t *testing.T) {
	// The API does not say whether an instance runs MySQL or Postgres, so an
	// edge would have to guess a provider and would never merge with a real
	// asset. The instance is metadata only.
	result := discover(t, newFakeAPI(), nil)

	for _, edge := range result.Lineage {
		assert.NotContains(t, edge.Source, "orders-db")
		assert.NotContains(t, edge.Target, "orders-db")
	}
}

func TestDiscover_EmitsNoEdgeForASecretOrNFSVolume(t *testing.T) {
	result := discover(t, newFakeAPI(), pluginsdk.RawConfig{"include_jobs": false})

	require.Len(t, result.Lineage, 1)
	assert.Equal(t, "mrn://bucket/gcs/order-archive", result.Lineage[0].Source)
}

func TestDiscover_RecordsASourceForEveryAsset(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	for _, asset := range result.Assets {
		require.Len(t, asset.Sources, 1)
		assert.Equal(t, "Cloud Run", asset.Sources[0].Name)
		assert.Equal(t, 1, asset.Sources[0].Priority)
	}
}

func TestDiscover_InterpolatesTagsFromMetadata(t *testing.T) {
	result := discover(t, newFakeAPI(), pluginsdk.RawConfig{
		"include_jobs": false,
		"tags":         []string{"cloudrun", "region-${location}"},
	})

	assert.Equal(t, []string{"cloudrun", "region-europe-west1"}, result.Assets[0].Tags)
}

func TestDiscover_FailsWhenTheAPIIsUnreachable(t *testing.T) {
	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"project_id":   testProject,
		"endpoint":     "http://127.0.0.1:1",
		"disable_auth": true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing services")
}
