// Package eks discovers Kubernetes assets from Amazon EKS clusters,
// authenticating with AWS IAM. Discovery itself is the shared engine
// from the kubernetes plugin; this package only supplies EKS auth and
// connection details.
package eks

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/marmotdata/marmot/plugins/kubernetes/kubernetes"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// eksTokenPrefix is the scheme EKS expects on presigned-URL bearer tokens.
const eksTokenPrefix = "k8s-aws-v1."

// Config for the EKS plugin: the shared discovery options plus AWS
// credentials and the cluster name.
//
// The endpoint and CA certificate are looked up from the EKS API using
// the cluster name and region. Only the credentials part of AWSConfig
// is used here; the tag-conversion options do not apply because EKS
// discovery produces Kubernetes assets, not tagged AWS resources.
type Config struct {
	kubernetes.DiscoveryConfig `json:",inline"`

	Credentials pluginsdk.AWSCredentials `json:"credentials" description:"AWS credentials configuration"`

	EKSClusterName string `json:"eks_cluster_name" label:"EKS Cluster Name" description:"EKS cluster name" validate:"required"`
}

// Example configuration for the plugin
var _ = `
eks_cluster_name: "prod"
credentials:
  region: "eu-west-1"
tags:
  - "kubernetes"
  - "eks"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "eks",
		Name:        "Elastic Kubernetes Service",
		Description: "Discover namespaces, services, workloads, and cron jobs from Amazon EKS clusters",
		Icon:        "eks",
		Category:    "compute",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source implements the EKS plugin.
type Source struct {
	config *Config
	client k8s.Interface
}

// Validate validates the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if err := config.DiscoveryConfig.Validate(); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover mints an EKS IAM token, builds a client, and runs the shared
// Kubernetes discovery engine.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	// The cluster name is a sensible default prefix for asset names, so
	// multi-cluster catalogs stay readable.
	if config.ClusterName == "" {
		config.ClusterName = config.EKSClusterName
	}
	s.config = config

	// Record where the cluster lives so assets are identifiable as EKS.
	cloudMetadata := map[string]interface{}{"cloud": "EKS"}
	if config.Credentials.Region != "" {
		cloudMetadata["aws_region"] = config.Credentials.Region
	}
	var clusterMetadata map[string]interface{}

	if s.client == nil {
		client, arn, err := newClient(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("creating EKS client: %w", err)
		}
		s.client = client

		// The ARN is the canonical cluster identifier and carries the
		// account and region, so record it and stamp the account on
		// every asset for multi-account context.
		if arn != "" {
			clusterMetadata = map[string]interface{}{"cluster_arn": arn}
			region, account := parseARN(arn)
			if region != "" {
				cloudMetadata["aws_region"] = region
			}
			if account != "" {
				cloudMetadata["aws_account_id"] = account
			}
		}
	}

	var links []pluginsdk.AssetExternalLink
	if region, _ := cloudMetadata["aws_region"].(string); region != "" {
		consoleURL := fmt.Sprintf("https://%s.console.aws.amazon.com/eks/home?region=%s#/clusters/%s",
			region, region, config.EKSClusterName)
		links = []pluginsdk.AssetExternalLink{{Name: "AWS console", URL: consoleURL}}
	}

	return kubernetes.NewDiscoverer(s.client, &config.DiscoveryConfig).
		WithMetadata(cloudMetadata).
		WithClusterMetadata(clusterMetadata).
		WithClusterExternalLinks(links).
		Discover(ctx)
}

// newClient resolves the endpoint and CA certificate from the EKS API,
// then builds a Kubernetes client using an IAM-derived bearer token. It
// also returns the cluster ARN.
func newClient(ctx context.Context, config *Config) (k8s.Interface, string, error) {
	awsConfig := pluginsdk.AWSConfig{Credentials: config.Credentials}
	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("loading AWS credentials: %w", err)
	}

	endpoint, caCert, arn, err := lookupCluster(ctx, awsCfg, config.EKSClusterName)
	if err != nil {
		return nil, "", fmt.Errorf("looking up EKS cluster: %w", err)
	}

	token, err := eksToken(ctx, awsCfg, config.EKSClusterName)
	if err != nil {
		return nil, "", fmt.Errorf("minting EKS token: %w", err)
	}

	restConfig := &rest.Config{
		Host:        endpoint,
		BearerToken: token,
	}
	if len(caCert) > 0 {
		restConfig.TLSClientConfig = rest.TLSClientConfig{CAData: caCert}
	}
	client, err := k8s.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}
	return client, arn, nil
}

// lookupCluster resolves a cluster's API server endpoint, CA
// certificate, and ARN from the EKS API.
func lookupCluster(ctx context.Context, awsCfg aws.Config, clusterName string) (endpoint string, caCert []byte, arn string, err error) {
	out, err := eks.NewFromConfig(awsCfg).DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		return "", nil, "", err
	}
	cluster := out.Cluster

	if cluster.Endpoint == nil || *cluster.Endpoint == "" {
		return "", nil, "", fmt.Errorf("cluster %q has no endpoint", clusterName)
	}
	if cluster.CertificateAuthority == nil || cluster.CertificateAuthority.Data == nil {
		return "", nil, "", fmt.Errorf("cluster %q has no CA certificate", clusterName)
	}
	ca, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return "", nil, "", fmt.Errorf("decoding cluster CA certificate: %w", err)
	}

	return *cluster.Endpoint, ca, aws.ToString(cluster.Arn), nil
}

// parseARN pulls the region and account ID out of an ARN, whose format
// is arn:partition:service:region:account-id:resource.
func parseARN(arn string) (region, account string) {
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return "", ""
	}
	return parts[3], parts[4]
}

// eksToken mints an EKS bearer token: a presigned STS GetCallerIdentity
// URL carrying the cluster name in a signed header, which the EKS API
// server validates against IAM. Credentials come from the ambient AWS
// chain (IRSA, EKS Pod Identity, instance profile, or static keys).
func eksToken(ctx context.Context, awsCfg aws.Config, clusterName string) (string, error) {
	presign := sts.NewPresignClient(sts.NewFromConfig(awsCfg))
	req, err := presign.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.PresignOptions) {
		o.ClientOptions = append(o.ClientOptions, func(so *sts.Options) {
			// Sign the cluster name into the request so EKS can bind the
			// token to this cluster.
			so.APIOptions = append(so.APIOptions, smithyhttp.SetHeaderValue("x-k8s-aws-id", clusterName))
		})
	})
	if err != nil {
		return "", fmt.Errorf("presigning STS request: %w", err)
	}

	return eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(req.URL)), nil
}
