// Package gcs discovers buckets from Google Cloud Storage.
package gcs

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// Config for the Google Cloud Storage plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection options
	ProjectID       string `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	CredentialsFile string `json:"credentials_file,omitempty" description:"Path to service account JSON file"`
	CredentialsJSON string `json:"credentials_json,omitempty" description:"Service account JSON content" sensitive:"true"`
	Endpoint        string `json:"endpoint,omitempty" description:"Custom endpoint URL (for fake-gcs-server or other emulators)"`
	DisableAuth     bool   `json:"disable_auth,omitempty" description:"Disable authentication (for local emulators)"`

	// Discovery options
	IncludeMetadata    bool `json:"include_metadata" description:"Include bucket metadata like labels" default:"true"`
	IncludeObjectCount bool `json:"include_object_count" description:"Count objects in each bucket (can be slow for large buckets)" default:"false"`
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "gcs",
		Name:        "Google Cloud Storage",
		Description: "Discover buckets from Google Cloud Storage",
		Icon:        "gcs",
		Category:    "storage",
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

type Source struct {
	config *Config
	client *storage.Client
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
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

	var assets []pluginsdk.Asset

	for _, bucket := range buckets {
		asset, err := s.createBucketAsset(ctx, bucket)
		if err != nil {
			continue
		}
		assets = append(assets, asset)
	}

	return &pluginsdk.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) createClient(ctx context.Context) (*storage.Client, error) {
	var opts []option.ClientOption

	if s.config.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(s.config.Endpoint))
	}

	switch {
	case s.config.DisableAuth:
		opts = append(opts, option.WithoutAuthentication())
	case s.config.CredentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(s.config.CredentialsJSON)))
	case s.config.CredentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(s.config.CredentialsFile))
	}

	return storage.NewClient(ctx, opts...)
}

func (s *Source) discoverBuckets(ctx context.Context) ([]*storage.BucketAttrs, error) {
	var buckets []*storage.BucketAttrs

	it := s.client.Buckets(ctx, s.config.ProjectID)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterating buckets: %w", err)
		}
		buckets = append(buckets, attrs)
	}

	return buckets, nil
}

func (s *Source) createBucketAsset(ctx context.Context, bucket *storage.BucketAttrs) (pluginsdk.Asset, error) {
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

	if len(bucket.Lifecycle.Rules) > 0 {
		metadata["lifecycle_rules_count"] = len(bucket.Lifecycle.Rules)
	}

	if s.config.IncludeObjectCount {
		count, err := s.countObjects(ctx, bucketName)
		if err == nil {
			metadata["object_count"] = count
		}
	}

	mrnValue := mrn.New("Bucket", "GCS", bucketName)

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:      &bucketName,
		MRN:       &mrnValue,
		Type:      "Bucket",
		Providers: []string{"GCS"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []pluginsdk.AssetSource{{
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
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}
