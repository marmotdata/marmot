// Package gke discovers Kubernetes assets from Google Kubernetes Engine
// clusters, authenticating with Google Cloud IAM. Discovery itself is
// the shared engine from the kubernetes plugin; this package only
// supplies GKE auth and connection details.
package gke

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/marmotdata/marmot/plugins/kubernetes/kubernetes"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"golang.org/x/oauth2"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// gkeScope is the OAuth scope for authenticating to the GKE API server
// and the Google Kubernetes Engine management API.
const gkeScope = "https://www.googleapis.com/auth/cloud-platform"

// Config for the GKE plugin: the shared discovery options plus Google
// Cloud credentials and the cluster identity.
//
// The endpoint and CA certificate are looked up from the GKE management
// API using the project, location, and cluster name.
type Config struct {
	kubernetes.DiscoveryConfig `json:",inline"`
	pluginsdk.GCPConfig        `json:",inline"`

	ProjectID string `json:"project_id" label:"Project ID" description:"GCP project ID" validate:"required"`
	Location  string `json:"location" description:"Cluster region or zone, for example us-central1" validate:"required"`
	Cluster   string `json:"cluster" description:"GKE cluster name" validate:"required"`
}

// Example configuration for the plugin
var _ = `
project_id: "my-project"
location: "us-central1"
cluster: "autopilot-cluster-1"
tags:
  - "kubernetes"
  - "gke"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "gke",
		Name:        "Google Kubernetes Engine",
		Description: "Discover namespaces, services, workloads, and cron jobs from Google GKE clusters",
		Icon:        "gke",
		Category:    "compute",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source implements the GKE plugin.
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

// Discover builds a client and runs the shared Kubernetes discovery engine.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	// The cluster name is a sensible default prefix for asset names, so
	// multi-cluster catalogs stay readable.
	if config.ClusterName == "" {
		config.ClusterName = config.Cluster
	}
	s.config = config

	if s.client == nil {
		client, err := newClient(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("creating GKE client: %w", err)
		}
		s.client = client
	}

	// Record where the cluster lives so assets are identifiable as GKE.
	cloudMetadata := map[string]interface{}{
		"cloud":        "GKE",
		"gcp_project":  config.ProjectID,
		"gcp_location": config.Location,
	}

	consoleURL := fmt.Sprintf("https://console.cloud.google.com/kubernetes/clusters/details/%s/%s/details?project=%s",
		config.Location, config.Cluster, config.ProjectID)
	links := []pluginsdk.AssetExternalLink{{Name: "Google Cloud console", URL: consoleURL}}

	return kubernetes.NewDiscoverer(s.client, &config.DiscoveryConfig).
		WithMetadata(cloudMetadata).
		WithClusterExternalLinks(links).
		Discover(ctx)
}

// newClient resolves the endpoint and CA certificate from the GKE
// management API, then builds a Kubernetes client using a Google OAuth
// bearer token.
func newClient(ctx context.Context, config *Config) (k8s.Interface, error) {
	tokenSource, err := config.GCPConfig.TokenSource(ctx, gkeScope)
	if err != nil {
		return nil, err
	}

	endpoint, caCert, err := lookupCluster(ctx, tokenSource, config)
	if err != nil {
		return nil, fmt.Errorf("looking up GKE cluster: %w", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("fetching Google token: %w", err)
	}

	restConfig := &rest.Config{
		Host:        "https://" + endpoint,
		BearerToken: token.AccessToken,
	}
	if len(caCert) > 0 {
		restConfig.TLSClientConfig = rest.TLSClientConfig{CAData: caCert}
	}
	return k8s.NewForConfig(restConfig)
}

// lookupCluster resolves a cluster's API server endpoint and CA
// certificate from the GKE management API.
func lookupCluster(ctx context.Context, tokenSource oauth2.TokenSource, config *Config) (endpoint string, caCert []byte, err error) {
	svc, err := container.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", nil, fmt.Errorf("creating GKE management client: %w", err)
	}

	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", config.ProjectID, config.Location, config.Cluster)
	cluster, err := svc.Projects.Locations.Clusters.Get(name).Context(ctx).Do()
	if err != nil {
		return "", nil, err
	}

	if cluster.Endpoint == "" {
		return "", nil, fmt.Errorf("cluster %q has no reachable endpoint", config.Cluster)
	}

	if cluster.MasterAuth == nil || cluster.MasterAuth.ClusterCaCertificate == "" {
		return "", nil, fmt.Errorf("cluster %q has no CA certificate", config.Cluster)
	}
	ca, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return "", nil, fmt.Errorf("decoding cluster CA certificate: %w", err)
	}

	return cluster.Endpoint, ca, nil
}
