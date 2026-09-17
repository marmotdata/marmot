package vertexai

import (
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// featureGroupAssets catalogues the feature store. A feature group is a
// set of columns keyed by an entity id, so it is filed as a Dataset with
// its features as the schema.
func (s *Source) featureGroupAssets(items []scannedFeatureGroup, edges *edgeSet) ([]pluginsdk.Asset, []pluginsdk.Statistic) {
	assets := make([]pluginsdk.Asset, 0, len(items))
	var statistics []pluginsdk.Statistic

	for _, item := range items {
		group := item.resource
		id := s.locate(group.Name, item.location)

		// A feature group id is chosen by the user and is unique within a
		// project and location, so it needs no collision handling.
		name := id.id
		if name == "" {
			log.Warn().Str("resource_name", group.Name).Msg("Skipping a feature group with no id")
			continue
		}

		columns := featureColumns(item.features)
		metadata := s.featureGroupMetadata(group, id, columns, item.featuresRead)

		asset := s.newAsset("Dataset", name, group.Description, metadata)
		if len(columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("feature_group", name).Msg("Failed to set feature group columns")
			}
		}
		assets = append(assets, asset)

		// A group whose features were denied has an unknown count, and a
		// zero would read as an empty group.
		if item.featuresRead {
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   *asset.MRN,
				MetricName: "asset.column_count",
				Value:      float64(len(columns)),
			})
		}

		// A Vertex feature group reads from a BigQuery table that already
		// exists, so the table feeds the group. This is the opposite
		// direction to SageMaker, where a feature group writes its offline
		// store out to Glue.
		if table := bigQueryTable(bigQuerySourceURI(group)); table != "" {
			edges.add(mrn.New("Table", "BigQuery", table), *asset.MRN, "FEEDS")
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered feature groups")
	return assets, statistics
}

func (s *Source) featureGroupMetadata(group *aiplatform.GoogleCloudAiplatformV1FeatureGroup, id resourceName, columns []pluginsdk.Column, featuresRead bool) map[string]any {
	metadata := commonMetadata(id, group.CreateTime, group.UpdateTime, group.Labels)

	// A feature group has no display name of its own; its id is what the
	// console and every other Vertex surface show.
	putString(metadata, "display_name", id.id)
	putString(metadata, "feature_group_id", id.id)

	if bigQuery := group.BigQuery; bigQuery != nil {
		putStrings(metadata, "entity_id_columns", bigQuery.EntityIdColumns)
		putBool(metadata, "dense", bigQuery.Dense)
		putBool(metadata, "static_data_source", bigQuery.StaticDataSource)
	}
	putString(metadata, "big_query_source_uri", bigQuerySourceURI(group))

	putString(metadata, "service_account_email", group.ServiceAccountEmail)
	if featuresRead {
		metadata["feature_count"] = len(columns)
	}

	return metadata
}

// bigQuerySourceURI is the table a feature group reads its features from.
func bigQuerySourceURI(group *aiplatform.GoogleCloudAiplatformV1FeatureGroup) string {
	if group.BigQuery == nil || group.BigQuery.BigQuerySource == nil {
		return ""
	}
	return group.BigQuery.BigQuerySource.InputUri
}

// featureColumns turns a feature group's features into catalog columns,
// sorted by name so the schema does not reshuffle between runs.
func featureColumns(features []*aiplatform.GoogleCloudAiplatformV1Feature) []pluginsdk.Column {
	columns := make([]pluginsdk.Column, 0, len(features))

	for _, feature := range features {
		name := resourceID(feature.Name)
		if name == "" {
			continue
		}
		columns = append(columns, pluginsdk.Column{
			Name:        name,
			DataType:    strings.ToLower(feature.ValueType),
			Nullable:    true,
			Description: feature.Description,
		})
	}

	sort.Slice(columns, func(i, j int) bool { return columns[i].Name < columns[j].Name })
	return columns
}
