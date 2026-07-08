// Package dynamodb discovers tables from Amazon DynamoDB accounts.
package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "dynamodb",
		Name:        "AWS DynamoDB",
		Description: "Discover DynamoDB tables from AWS accounts",
		Icon:        "dynamodb",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the DynamoDB plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	AWSConfig            `json:",inline"`
}

// Example configuration for the plugin.
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
	client *awsdynamodb.Client
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	awsCfg, err := extractAWSConfig(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	sdkCfg, err := awsCfg.newAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = awsdynamodb.NewFromConfig(sdkCfg)

	tableNames, err := s.discoverTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}

	var assets []pluginsdk.Asset
	for _, tableName := range tableNames {
		a, err := s.createTableAsset(ctx, tableName)
		if err != nil {
			log.Warn().Err(err).Str("table", tableName).Msg("Failed to create asset for table")
			continue
		}
		assets = append(assets, a)
	}

	return &pluginsdk.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) discoverTables(ctx context.Context) ([]string, error) {
	var tableNames []string
	paginator := awsdynamodb.NewListTablesPaginator(s.client, &awsdynamodb.ListTablesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tables: %w", err)
		}
		tableNames = append(tableNames, output.TableNames...)
	}

	return tableNames, nil
}

func (s *Source) createTableAsset(ctx context.Context, tableName string) (pluginsdk.Asset, error) {
	metadata := make(map[string]interface{})

	describeOutput, err := s.client.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: &tableName,
	})
	if err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("describing table: %w", err)
	}

	table := describeOutput.Table

	if s.config.TagsToMetadata {
		tagsOutput, err := s.client.ListTagsOfResource(ctx, &awsdynamodb.ListTagsOfResourceInput{
			ResourceArn: table.TableArn,
		})
		if err != nil {
			log.Warn().Err(err).Str("table", tableName).Msg("Failed to get table tags")
		} else {
			tagMap := make(map[string]string)
			for _, tag := range tagsOutput.Tags {
				tagMap[*tag.Key] = *tag.Value
			}
			metadata = processAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
		}
	}

	if table.TableArn != nil {
		metadata["table_arn"] = *table.TableArn
	}
	metadata["table_status"] = string(table.TableStatus)
	if table.CreationDateTime != nil {
		metadata["creation_date"] = table.CreationDateTime.Format(time.RFC3339)
	}

	if table.TableClassSummary != nil {
		metadata["table_class"] = string(table.TableClassSummary.TableClass)
	}

	if table.BillingModeSummary != nil {
		metadata["billing_mode"] = string(table.BillingModeSummary.BillingMode)
	}

	if table.ProvisionedThroughput != nil {
		if table.ProvisionedThroughput.ReadCapacityUnits != nil {
			metadata["read_capacity_units"] = *table.ProvisionedThroughput.ReadCapacityUnits
		}
		if table.ProvisionedThroughput.WriteCapacityUnits != nil {
			metadata["write_capacity_units"] = *table.ProvisionedThroughput.WriteCapacityUnits
		}
	}

	if len(table.KeySchema) > 0 {
		var keyParts []string
		for _, key := range table.KeySchema {
			keyParts = append(keyParts, fmt.Sprintf("%s(%s)", *key.AttributeName, string(key.KeyType)))
		}
		metadata["key_schema"] = strings.Join(keyParts, ", ")
	}

	if len(table.AttributeDefinitions) > 0 {
		var attrParts []string
		for _, attr := range table.AttributeDefinitions {
			attrParts = append(attrParts, fmt.Sprintf("%s(%s)", *attr.AttributeName, string(attr.AttributeType)))
		}
		metadata["attribute_definitions"] = strings.Join(attrParts, ", ")
	}

	metadata["gsi_count"] = len(table.GlobalSecondaryIndexes)
	metadata["lsi_count"] = len(table.LocalSecondaryIndexes)

	if table.StreamSpecification != nil {
		metadata["stream_enabled"] = boolToString(table.StreamSpecification.StreamEnabled)
		if table.StreamSpecification.StreamViewType != "" {
			metadata["stream_view_type"] = string(table.StreamSpecification.StreamViewType)
		}
	}

	if table.SSEDescription != nil {
		metadata["encryption_status"] = string(table.SSEDescription.Status)
		metadata["encryption_type"] = string(table.SSEDescription.SSEType)
	}

	if table.TableSizeBytes != nil {
		metadata["table_size_bytes"] = *table.TableSizeBytes
	}
	if table.ItemCount != nil {
		metadata["item_count"] = *table.ItemCount
	}

	if table.DeletionProtectionEnabled != nil {
		metadata["deletion_protection"] = boolToString(table.DeletionProtectionEnabled)
	}

	if len(table.Replicas) > 0 {
		var replicaRegions []string
		for _, replica := range table.Replicas {
			if replica.RegionName != nil {
				replicaRegions = append(replicaRegions, *replica.RegionName)
			}
		}
		metadata["global_table_replicas"] = strings.Join(replicaRegions, ", ")
	}

	ttlOutput, err := s.client.DescribeTimeToLive(ctx, &awsdynamodb.DescribeTimeToLiveInput{
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

	backupsOutput, err := s.client.DescribeContinuousBackups(ctx, &awsdynamodb.DescribeContinuousBackupsInput{
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

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:      &tableName,
		MRN:       &mrnValue,
		Type:      "Table",
		Providers: []string{"DynamoDB"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []pluginsdk.AssetSource{{
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
