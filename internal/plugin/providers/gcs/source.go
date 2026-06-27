// +marmot:name=Google Cloud Storage
// +marmot:description=Discovers buckets from Google Cloud Storage.
// +marmot:status=experimental
// +marmot:features=Assets
package gcs

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for Google Cloud Storage plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection options
	ProjectID                 string `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	ImpersonateServiceAccount string `json:"impersonate_service_account,omitempty" description:"Email of a service account to impersonate."`
	CredentialsFile           string `json:"credentials_file,omitempty" description:"Path to a service account JSON key file."`
	CredentialsJSON           string `json:"credentials_json,omitempty" sensitive:"true" description:"Service account JSON key content."`
	Endpoint                  string `json:"endpoint,omitempty" description:"Custom endpoint URL (for fake-gcs-server or other emulators)"`
	DisableAuth               bool   `json:"disable_auth,omitempty" description:"Disable authentication (for local emulators)"`

	// Discovery options
	IncludeMetadata    bool `json:"include_metadata" description:"Include bucket metadata like labels" default:"true"`
	IncludeObjectCount bool `json:"include_object_count" description:"Count objects in each bucket (can be slow for large buckets)" default:"false"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
project_id: "my-gcp-project"
# Authentication uses Application Default Credentials by default.
include_metadata: true
include_object_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "gcs"
  - "storage"
`

type Source struct {
	config *Config
	client *storage.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	client, err := s.createClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCS client: %w", err)
	}
	defer client.Close()
	s.client = client

	buckets, err := s.discoverBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering buckets: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for _, bucket := range buckets {
		asset, err := s.createBucketAsset(ctx, bucket)
		if err != nil {
			log.Warn().Err(err).Str("bucket", bucket.Name).Msg("Failed to create asset for bucket")
			continue
		}
		assets = append(assets, asset)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) createClient(ctx context.Context) (*storage.Client, error) {
	authOpts := s.config.authOptions()

	log.Debug().
		Str("credential_source", s.config.credentialSource()).
		Str("impersonate", s.config.ImpersonateServiceAccount).
		Msg("Authenticating to GCS")

	clientOpts := authOpts
	if s.config.ImpersonateServiceAccount != "" {
		ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: s.config.ImpersonateServiceAccount,
			Scopes:          []string{storage.ScopeReadOnly},
		}, authOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating impersonated token source for %q: %w", s.config.ImpersonateServiceAccount, err)
		}
		clientOpts = []option.ClientOption{option.WithTokenSource(ts)}
	}

	if s.config.Endpoint != "" {
		clientOpts = append(clientOpts, option.WithEndpoint(s.config.Endpoint))
	}

	return storage.NewClient(ctx, clientOpts...)
}

// authOptions returns the client options that select how the plugin authenticates.
// When none of the explicit credential fields are set, it returns nil, so the client
// falls back to Application Default Credentials.
func (c *Config) authOptions() []option.ClientOption {
	switch {
	case c.DisableAuth:
		return []option.ClientOption{option.WithoutAuthentication()}
	case c.CredentialsJSON != "":
		return []option.ClientOption{option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(c.CredentialsJSON))}
	case c.CredentialsFile != "":
		return []option.ClientOption{option.WithAuthCredentialsFile(option.ServiceAccount, c.CredentialsFile)}
	default:
		return nil
	}
}

// credentialSource describes, for logging, how the plugin authenticates. It mirrors
// the precedence in authOptions.
func (c *Config) credentialSource() string {
	switch {
	case c.DisableAuth:
		return "disabled"
	case c.CredentialsJSON != "":
		return "Service Account JSON"
	case c.CredentialsFile != "":
		return "Service Account file"
	default:
		return "application default credentials"
	}
}

func (s *Source) discoverBuckets(ctx context.Context) ([]*storage.BucketAttrs, error) {
	var buckets []*storage.BucketAttrs

	it := s.client.Buckets(ctx, s.config.ProjectID)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterating buckets: %w", err)
		}
		buckets = append(buckets, attrs)
	}

	return buckets, nil
}

func (s *Source) createBucketAsset(ctx context.Context, bucket *storage.BucketAttrs) (asset.Asset, error) {
	bucketName := bucket.Name

	metadata := make(map[string]interface{})
	metadata["bucket_name"] = bucketName
	metadata["location"] = bucket.Location
	metadata["location_type"] = bucket.LocationType
	metadata["storage_class"] = bucket.StorageClass
	metadata["created"] = bucket.Created.Format(time.RFC3339)

	if bucket.VersioningEnabled {
		metadata["versioning"] = "enabled"
	} else {
		metadata["versioning"] = "disabled"
	}

	if bucket.RequesterPays {
		metadata["requester_pays"] = true
	}

	if bucket.DefaultEventBasedHold {
		metadata["default_event_based_hold"] = true
	}

	if bucket.RetentionPolicy != nil {
		metadata["retention_period_seconds"] = bucket.RetentionPolicy.RetentionPeriod.Seconds()
	}

	if bucket.Encryption != nil && bucket.Encryption.DefaultKMSKeyName != "" {
		metadata["encryption"] = "customer-managed"
		metadata["kms_key"] = bucket.Encryption.DefaultKMSKeyName
	} else {
		metadata["encryption"] = "google-managed"
	}

	if bucket.Logging != nil && bucket.Logging.LogBucket != "" {
		metadata["logging_enabled"] = true
		metadata["log_bucket"] = bucket.Logging.LogBucket
	}

	if s.config.IncludeMetadata && len(bucket.Labels) > 0 {
		for key, value := range bucket.Labels {
			metadata["label_"+key] = value
		}
	}

	if bucket.Lifecycle.Rules != nil && len(bucket.Lifecycle.Rules) > 0 {
		metadata["lifecycle_rules_count"] = len(bucket.Lifecycle.Rules)
	}

	if s.config.IncludeObjectCount {
		count, err := s.countObjects(ctx, bucketName)
		if err != nil {
			log.Warn().Err(err).Str("bucket", bucketName).Msg("Failed to count objects")
		} else {
			metadata["object_count"] = count
		}
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:      &bucketName,
		MRN:       new(mrn.New("Bucket", "GCS", bucketName)),
		Type:      "Bucket",
		Providers: []string{"GCS"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []asset.AssetSource{{
			Name:       "GCS",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

func (s *Source) countObjects(ctx context.Context, bucketName string) (int64, error) {
	var count int64

	it := s.client.Bucket(bucketName).Objects(ctx, nil)
	for {
		_, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "gcs",
		Name:        "Google Cloud Storage",
		Description: "Discover buckets from Google Cloud Storage",
		Icon:        "gcs",
		Category:    "storage",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register GCS plugin")
	}
}
