package cloudrun

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	run "google.golang.org/api/run/v2"
)

func TestParseResourceName_SplitsAServiceName(t *testing.T) {
	parsed, err := parseResourceName("projects/acme/locations/europe-west1/services/checkout-api")
	require.NoError(t, err)

	assert.Equal(t, "acme", parsed.Project)
	assert.Equal(t, "europe-west1", parsed.Location)
	assert.Equal(t, "checkout-api", parsed.ID)
}

func TestParseResourceName_SplitsAJobName(t *testing.T) {
	parsed, err := parseResourceName("projects/acme/locations/us-central1/jobs/nightly-export")
	require.NoError(t, err)

	assert.Equal(t, "us-central1", parsed.Location)
	assert.Equal(t, "nightly-export", parsed.ID)
}

func TestParseResourceName_RejectsAMalformedName(t *testing.T) {
	_, err := parseResourceName("projects/acme/services/checkout-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected Cloud Run resource name")
}

func TestParseResourceName_RejectsAnExecutionName(t *testing.T) {
	// An execution name has two more segments, and only its bare id is ever
	// used, so it must not be mistaken for a service or a job.
	_, err := parseResourceName("projects/acme/locations/europe-west1/jobs/nightly-export/executions/nightly-export-x9k2t")
	require.Error(t, err)
}

func TestParseResourceName_RejectsAnEmptyLocation(t *testing.T) {
	_, err := parseResourceName("projects/acme/locations//services/checkout-api")
	require.Error(t, err)
}

func TestResourceID_ReturnsTheLastSegment(t *testing.T) {
	assert.Equal(t, "checkout-api-00042-abc",
		resourceID("projects/acme/locations/europe-west1/services/checkout-api/revisions/checkout-api-00042-abc"))
}

func TestResourceID_LeavesABareIDAlone(t *testing.T) {
	assert.Equal(t, "checkout-api-00042-abc", resourceID("checkout-api-00042-abc"))
}

func TestResourceID_IsEmptyForAnEmptyName(t *testing.T) {
	assert.Equal(t, "", resourceID(""))
}

func TestFormatTraffic_WritesASingleLatestTargetAsLatest(t *testing.T) {
	statuses := []*run.GoogleCloudRunV2TrafficTargetStatus{
		{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", Percent: 100},
	}

	assert.Equal(t, "latest=100", formatTraffic(statuses, nil))
}

func TestFormatTraffic_WritesASplitAcrossNamedRevisions(t *testing.T) {
	statuses := []*run.GoogleCloudRunV2TrafficTargetStatus{
		{Revision: "projects/acme/locations/europe-west1/services/api/revisions/rev-a", Percent: 90},
		{Revision: "projects/acme/locations/europe-west1/services/api/revisions/rev-b", Percent: 10},
	}

	assert.Equal(t, "rev-a=90,rev-b=10", formatTraffic(statuses, nil))
}

func TestFormatTraffic_FallsBackToTheRequestedSplit(t *testing.T) {
	// TrafficStatuses is what Cloud Run actually serves, but a service that
	// has not converged yet only has the requested Traffic.
	targets := []*run.GoogleCloudRunV2TrafficTarget{
		{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", Percent: 100},
	}

	assert.Equal(t, "latest=100", formatTraffic(nil, targets))
}

func TestFormatTraffic_IsEmptyWhenThereIsNoSplit(t *testing.T) {
	assert.Equal(t, "", formatTraffic(nil, nil))
}

func TestDiscover_RecordsTheServedTrafficSplit(t *testing.T) {
	result := discover(t, newFakeAPI(), nil)

	metadata := assetNamed(t, result, "europe-west1/checkout-api").Metadata
	assert.Equal(t, "checkout-api-00042-abc=90,checkout-api-00043-def=10", metadata["traffic"])
}
