package eks

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_LookupConnection(t *testing.T) {
	// Only the cluster name and region are needed; host and CA are
	// resolved from the EKS API.
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"eks_cluster_name": "prod",
		"credentials":      map[string]interface{}{"region": "eu-west-1"},
	})
	require.NoError(t, err)
}

func TestValidate_RequiresClusterName(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"credentials": map[string]interface{}{"region": "eu-west-1"},
	})
	require.Error(t, err)
}

func TestValidate_NothingToDiscover(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"eks_cluster_name":      "prod",
		"discover_namespaces":   false,
		"discover_services":     false,
		"discover_deployments":  false,
		"discover_statefulsets": false,
		"discover_cronjobs":     false,
		"discover_pods":         false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to discover")
}

func TestValidate_AppliesDiscoveryDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"eks_cluster_name": "prod"})
	require.NoError(t, err)
	assert.True(t, s.config.DiscoverServices)
	assert.True(t, s.config.DiscoverStatefulSets)
	assert.False(t, s.config.DiscoverPods)
}

func TestParseARN(t *testing.T) {
	region, account := parseARN("arn:aws:eks:eu-west-1:123456789012:cluster/prod")
	assert.Equal(t, "eu-west-1", region)
	assert.Equal(t, "123456789012", account)

	region, account = parseARN("not-an-arn")
	assert.Empty(t, region)
	assert.Empty(t, account)
}

func TestEKSToken_Format(t *testing.T) {
	// Static env credentials keep the presign fully offline and
	// deterministic; presigning never calls AWS.
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_REGION", "eu-west-1")

	awsConfig := pluginsdk.AWSConfig{Credentials: pluginsdk.AWSCredentials{Region: "eu-west-1"}}
	awsCfg, err := awsConfig.NewAWSConfig(context.Background())
	require.NoError(t, err)

	token, err := eksToken(context.Background(), awsCfg, "prod-cluster")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(token, "k8s-aws-v1."), "token must carry the EKS scheme prefix")

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, "k8s-aws-v1."))
	require.NoError(t, err)
	presignedURL := string(raw)

	assert.Contains(t, presignedURL, "sts.eu-west-1.amazonaws.com")
	assert.Contains(t, presignedURL, "Action=GetCallerIdentity")
	assert.Contains(t, presignedURL, "X-Amz-Credential")
	assert.Contains(t, presignedURL, "x-k8s-aws-id", "cluster name must be a signed header")
}
