package sagemaker

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// discoverFeatureGroups catalogues the feature store. A feature group is a
// set of columns keyed by a record identifier, so it is filed as a Dataset
// with its feature definitions as the schema.
func (s *Source) discoverFeatureGroups(ctx context.Context) ([]pluginsdk.Asset, []pluginsdk.LineageEdge, error) {
	summaries, err := s.listFeatureGroups(ctx)
	if err != nil {
		return nil, nil, err
	}

	var assets []pluginsdk.Asset
	var edges []pluginsdk.LineageEdge

	for _, summary := range summaries {
		name := deref(summary.FeatureGroupName)
		if name == "" {
			continue
		}

		asset, err := s.featureGroupAsset(ctx, name, summary)
		if err != nil {
			log.Warn().Err(err).Str("feature_group", name).Msg("Failed to describe feature group")
			continue
		}
		assets = append(assets, asset)
		edges = append(edges, offlineStoreEdges(asset)...)
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered feature groups")
	return assets, edges, nil
}

func (s *Source) featureGroupAsset(ctx context.Context, name string, summary types.FeatureGroupSummary) (pluginsdk.Asset, error) {
	detail, err := s.client.DescribeFeatureGroup(ctx, &sagemaker.DescribeFeatureGroupInput{
		FeatureGroupName: &name,
	})
	if err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("describing feature group: %w", err)
	}

	arn := deref(summary.FeatureGroupArn)
	if arn == "" {
		arn = deref(detail.FeatureGroupArn)
	}

	metadata := s.resourceTags(ctx, arn)
	if metadata == nil {
		metadata = map[string]any{}
	}

	if arn != "" {
		metadata["arn"] = arn
	}
	putString(metadata, "record_identifier", detail.RecordIdentifierFeatureName)
	putString(metadata, "event_time_feature", detail.EventTimeFeatureName)

	if detail.FeatureGroupStatus != "" {
		metadata["status"] = string(detail.FeatureGroupStatus)
	}

	description := deref(detail.Description)
	if description != "" {
		metadata["description"] = description
	}

	if online := detail.OnlineStoreConfig; online != nil && online.EnableOnlineStore != nil {
		metadata["online_store"] = *online.EnableOnlineStore
	}

	if offline := detail.OfflineStoreConfig; offline != nil {
		if offline.S3StorageConfig != nil {
			putString(metadata, "offline_store_s3_uri", offline.S3StorageConfig.S3Uri)
		}
		// The offline store is queryable through a Glue table, which the
		// Glue plugin catalogues separately.
		if catalog := offline.DataCatalogConfig; catalog != nil {
			database := deref(catalog.Database)
			table := deref(catalog.TableName)
			if database != "" && table != "" {
				metadata["glue_table"] = database + "." + table
			}
		}
	}

	putTime(metadata, "created_at", detail.CreationTime)
	if _, ok := metadata["created_at"]; !ok {
		putTime(metadata, "created_at", summary.CreationTime)
	}

	if s.region != "" {
		metadata["region"] = s.region
	}
	// No console link: the feature store is only browsable in SageMaker
	// Studio, and the classic console has no confirmed route to a group.

	asset := s.newAsset("Dataset", name, description, metadata)

	columns := featureColumns(detail.FeatureDefinitions, deref(detail.RecordIdentifierFeatureName), deref(detail.EventTimeFeatureName))
	if len(columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, columns); err != nil {
			log.Warn().Err(err).Str("feature_group", name).Msg("Failed to set feature group columns")
		}
	}

	return asset, nil
}

// featureColumns turns feature definitions into catalog columns. The record
// identifier is the group's key, and it and the event time are the two
// features every record must carry.
func featureColumns(definitions []types.FeatureDefinition, recordIdentifier, eventTime string) []pluginsdk.Column {
	var columns []pluginsdk.Column

	for _, d := range definitions {
		name := deref(d.FeatureName)
		if name == "" {
			continue
		}
		columns = append(columns, pluginsdk.Column{
			Name:       name,
			DataType:   string(d.FeatureType),
			Nullable:   name != recordIdentifier && name != eventTime,
			PrimaryKey: name == recordIdentifier,
		})
	}

	return columns
}

// offlineStoreEdges links a feature group to the storage its offline store
// writes to. Both endpoints belong to other plugins, so only the edges are
// emitted and the server drops the ones with nothing behind them.
func offlineStoreEdges(asset pluginsdk.Asset) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge

	if table, ok := asset.Metadata["glue_table"].(string); ok && table != "" {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: *asset.MRN,
			Target: mrn.New("Table", "Glue", bareTableName(table)),
			Type:   "PRODUCES",
		})
	}

	if uri, ok := asset.Metadata["offline_store_s3_uri"].(string); ok {
		if bucket := s3Bucket(uri); bucket != "" {
			edges = append(edges, pluginsdk.LineageEdge{
				Source: *asset.MRN,
				Target: mrn.New("Bucket", "S3", bucket),
				Type:   "PRODUCES",
			})
		}
	}

	return edges
}

// bareTableName drops the database qualifier from a "database.table" name.
// The Glue plugin names its table assets with the table alone.
func bareTableName(qualified string) string {
	if i := strings.LastIndex(qualified, "."); i >= 0 {
		return qualified[i+1:]
	}
	return qualified
}
