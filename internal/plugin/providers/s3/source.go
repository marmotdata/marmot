// +marmot:name=S3
// +marmot:description=This plugin discovers S3 buckets from AWS accounts.
// +marmot:status=experimental
// +marmot:features=Assets
package s3

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for S3 plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
credentials:
  region: "us-east-1" 
  id: "<aws-secret-id>"
  secret: "<aws-secret-key>"
tags:
  - "s3"
`

type Source struct {
	config *Config
	client *s3.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
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

	awsConfig, err := plugin.ExtractAWSConfig(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if awsCfg.BaseEndpoint != nil {
			log.Debug().Str("endpoint", *awsCfg.BaseEndpoint).Msg("Using custom endpoint with path-style addressing")
			o.UsePathStyle = true
			o.BaseEndpoint = awsCfg.BaseEndpoint
		}
	})

	buckets, err := s.discoverBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering buckets: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for _, bucket := range buckets {
		asset, err := s.createBucketAsset(ctx, bucket)
		if err != nil {
			log.Warn().Err(err).Str("bucket", *bucket.Name).Msg("Failed to create asset for bucket")
			continue
		}
		assets = append(assets, asset)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) discoverBuckets(ctx context.Context) ([]types.Bucket, error) {
	output, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}

	return output.Buckets, nil
}

func (s *Source) createBucketAsset(ctx context.Context, bucket types.Bucket) (asset.Asset, error) {
	bucketName := *bucket.Name

	locationOutput, err := s.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: &bucketName,
	})
	if err != nil {
		log.Warn().Err(err).Str("bucket", bucketName).Msg("Failed to get bucket location")
	}

	metadata := make(map[string]interface{})

	bucketArn := fmt.Sprintf("arn:aws:s3:::%s", bucketName)
	metadata["bucket_arn"] = bucketArn
	metadata["creation_date"] = bucket.CreationDate.Format(time.RFC3339)

	if locationOutput != nil {
		region := string(locationOutput.LocationConstraint)
		if region == "" {
			region = "us-east-1"
		}
		metadata["region"] = region
	}

	if versioningOutput, err := s.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["versioning"] = string(versioningOutput.Status)
	}

	if encryptionOutput, err := s.client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: &bucketName,
	}); err == nil && encryptionOutput.ServerSideEncryptionConfiguration != nil {
		metadata["encryption"] = "enabled"
	} else {
		metadata["encryption"] = "disabled"
	}

	if pabOutput, err := s.client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: &bucketName,
	}); err == nil && pabOutput.PublicAccessBlockConfiguration != nil {
		pab := pabOutput.PublicAccessBlockConfiguration
		metadata["public_access_block"] = fmt.Sprintf("BlockPublicAcls:%t,IgnorePublicAcls:%t,BlockPublicPolicy:%t,RestrictPublicBuckets:%t",
			aws.ToBool(pab.BlockPublicAcls), aws.ToBool(pab.IgnorePublicAcls), aws.ToBool(pab.BlockPublicPolicy), aws.ToBool(pab.RestrictPublicBuckets))
	}

	if notificationOutput, err := s.client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: &bucketName,
	}); err == nil {
		hasNotifications := len(notificationOutput.TopicConfigurations) > 0 ||
			len(notificationOutput.QueueConfigurations) > 0 ||
			len(notificationOutput.LambdaFunctionConfigurations) > 0
		if hasNotifications {
			metadata["notification_config"] = "enabled"
		} else {
			metadata["notification_config"] = "disabled"
		}
	}

	if _, err := s.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["lifecycle_config"] = "enabled"
	} else {
		metadata["lifecycle_config"] = "disabled"
	}

	if _, err := s.client.GetBucketReplication(ctx, &s3.GetBucketReplicationInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["replication_config"] = "enabled"
	} else {
		metadata["replication_config"] = "disabled"
	}

	if _, err := s.client.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["website_config"] = "enabled"
	} else {
		metadata["website_config"] = "disabled"
	}

	if loggingOutput, err := s.client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{
		Bucket: &bucketName,
	}); err == nil && loggingOutput.LoggingEnabled != nil {
		metadata["logging_config"] = "enabled"
	} else {
		metadata["logging_config"] = "disabled"
	}

	if accelerateOutput, err := s.client.GetBucketAccelerateConfiguration(ctx, &s3.GetBucketAccelerateConfigurationInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["accelerate_config"] = string(accelerateOutput.Status)
	}

	if paymentOutput, err := s.client.GetBucketRequestPayment(ctx, &s3.GetBucketRequestPaymentInput{
		Bucket: &bucketName,
	}); err == nil {
		metadata["request_payment_config"] = string(paymentOutput.Payer)
	}

	if s.config.TagsToMetadata {
		if tagsOutput, err := s.client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
			Bucket: &bucketName,
		}); err == nil {
			tagMap := make(map[string]string)
			for _, tag := range tagsOutput.TagSet {
				tagMap[*tag.Key] = *tag.Value
			}
			metadata = plugin.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
		}
	}

	mrnValue := mrn.New("Bucket", "S3", bucketName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:      &bucketName,
		MRN:       &mrnValue,
		Type:      "Bucket",
		Providers: []string{"S3"},
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "S3",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "s3",
		Name:        "AWS S3",
		Description: "Discover S3 buckets from AWS accounts",
		Icon:        "s3",
		Category:    "storage",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register S3 plugin")
	}
}
