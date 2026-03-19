// +marmot:name=DynamoDB
// +marmot:description=This plugin discovers DynamoDB tables from AWS accounts.
// +marmot:status=experimental
// +marmot:features=Assets
package dynamodb

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for DynamoDB plugin
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
  profile: "production"
  role: "<role>"
tags:
  - "aws"
`

type Source struct {
	config *Config
	client *dynamodb.Client
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

	s.client = dynamodb.NewFromConfig(awsCfg)

	tableNames, err := s.discoverTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}

	var assets []asset.Asset
	for _, tableName := range tableNames {
		a, err := s.createTableAsset(ctx, tableName)
		if err != nil {
			log.Warn().Err(err).Str("table", tableName).Msg("Failed to create asset for table")
			continue
		}
		assets = append(assets, a)
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) discoverTables(ctx context.Context) ([]string, error) {
	var tableNames []string
	paginator := dynamodb.NewListTablesPaginator(s.client, &dynamodb.ListTablesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tables: %w", err)
		}
		tableNames = append(tableNames, output.TableNames...)
	}

	return tableNames, nil
}

func (s *Source) createTableAsset(ctx context.Context, tableName string) (asset.Asset, error) {
	metadata := make(map[string]interface{})

	describeOutput, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &tableName,
	})
	if err != nil {
		return asset.Asset{}, fmt.Errorf("describing table: %w", err)
	}

	table := describeOutput.Table

	// Collect tags first (before other metadata, per convention)
	if s.config.AWSConfig != nil && s.config.TagsToMetadata {
		tagsOutput, err := s.client.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
			ResourceArn: table.TableArn,
		})
		if err != nil {
			log.Warn().Err(err).Str("table", tableName).Msg("Failed to get table tags")
		} else {
			tagMap := make(map[string]string)
			for _, tag := range tagsOutput.Tags {
				tagMap[*tag.Key] = *tag.Value
			}
			metadata = plugin.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
		}
	}

	// Table identity
	if table.TableArn != nil {
		metadata["table_arn"] = *table.TableArn
	}
	metadata["table_status"] = string(table.TableStatus)
	if table.CreationDateTime != nil {
		metadata["creation_date"] = table.CreationDateTime.Format(time.RFC3339)
	}

	// Table class
	if table.TableClassSummary != nil {
		metadata["table_class"] = string(table.TableClassSummary.TableClass)
	}

	// Billing mode
	if table.BillingModeSummary != nil {
		metadata["billing_mode"] = string(table.BillingModeSummary.BillingMode)
	}

	// Provisioned throughput
	if table.ProvisionedThroughput != nil {
		if table.ProvisionedThroughput.ReadCapacityUnits != nil {
			metadata["read_capacity_units"] = *table.ProvisionedThroughput.ReadCapacityUnits
		}
		if table.ProvisionedThroughput.WriteCapacityUnits != nil {
			metadata["write_capacity_units"] = *table.ProvisionedThroughput.WriteCapacityUnits
		}
	}

	// Key schema
	if len(table.KeySchema) > 0 {
		var keyParts []string
		for _, key := range table.KeySchema {
			keyParts = append(keyParts, fmt.Sprintf("%s(%s)", *key.AttributeName, string(key.KeyType)))
		}
		metadata["key_schema"] = strings.Join(keyParts, ", ")
	}

	// Attribute definitions
	if len(table.AttributeDefinitions) > 0 {
		var attrParts []string
		for _, attr := range table.AttributeDefinitions {
			attrParts = append(attrParts, fmt.Sprintf("%s(%s)", *attr.AttributeName, string(attr.AttributeType)))
		}
		metadata["attribute_definitions"] = strings.Join(attrParts, ", ")
	}

	// Indexes
	metadata["gsi_count"] = len(table.GlobalSecondaryIndexes)
	metadata["lsi_count"] = len(table.LocalSecondaryIndexes)

	// Streams
	if table.StreamSpecification != nil {
		metadata["stream_enabled"] = boolToString(table.StreamSpecification.StreamEnabled)
		if table.StreamSpecification.StreamViewType != "" {
			metadata["stream_view_type"] = string(table.StreamSpecification.StreamViewType)
		}
	}

	// Encryption
	if table.SSEDescription != nil {
		metadata["encryption_status"] = string(table.SSEDescription.Status)
		metadata["encryption_type"] = string(table.SSEDescription.SSEType)
	}

	// Size and item count
	if table.TableSizeBytes != nil {
		metadata["table_size_bytes"] = *table.TableSizeBytes
	}
	if table.ItemCount != nil {
		metadata["item_count"] = *table.ItemCount
	}

	// Deletion protection
	if table.DeletionProtectionEnabled != nil {
		metadata["deletion_protection"] = boolToString(table.DeletionProtectionEnabled)
	}

	// Global table replicas
	if len(table.Replicas) > 0 {
		var replicaRegions []string
		for _, replica := range table.Replicas {
			if replica.RegionName != nil {
				replicaRegions = append(replicaRegions, *replica.RegionName)
			}
		}
		metadata["global_table_replicas"] = strings.Join(replicaRegions, ", ")
	}

	// TTL
	ttlOutput, err := s.client.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: &tableName,
	})
	if err != nil {
		log.Warn().Err(err).Str("table", tableName).Msg("Failed to get TTL description")
	} else if ttlOutput.TimeToLiveDescription != nil {
		metadata["ttl_status"] = string(ttlOutput.TimeToLiveDescription.TimeToLiveStatus)
		if ttlOutput.TimeToLiveDescription.AttributeName != nil {
			metadata["ttl_attribute"] = *ttlOutput.TimeToLiveDescription.AttributeName
		}
	}

	// Continuous backups / PITR
	backupsOutput, err := s.client.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: &tableName,
	})
	if err != nil {
		log.Warn().Err(err).Str("table", tableName).Msg("Failed to get continuous backups description")
	} else if backupsOutput.ContinuousBackupsDescription != nil {
		metadata["continuous_backups"] = string(backupsOutput.ContinuousBackupsDescription.ContinuousBackupsStatus)
		if backupsOutput.ContinuousBackupsDescription.PointInTimeRecoveryDescription != nil {
			metadata["pitr_status"] = string(backupsOutput.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus)
		}
	}

	mrnValue := mrn.New("Table", "DynamoDB", tableName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:      &tableName,
		MRN:       &mrnValue,
		Type:      "Table",
		Providers: []string{"DynamoDB"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []asset.AssetSource{{
			Name:       "DynamoDB",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

func boolToString(b *bool) string {
	if b != nil && *b {
		return "true"
	}
	return "false"
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "dynamodb",
		Name:        "AWS DynamoDB",
		Description: "Discover DynamoDB tables from AWS accounts",
		Icon:        "dynamodb",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register DynamoDB plugin")
	}
}
