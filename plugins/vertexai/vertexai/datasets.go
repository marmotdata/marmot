package vertexai

import (
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/aiplatform/v1"
)

// datasetAssets catalogues the managed datasets a project trains from, and
// links each one to the BigQuery table or Cloud Storage bucket it reads.
func (s *Source) datasetAssets(items []scanned[*aiplatform.GoogleCloudAiplatformV1Dataset], edges *edgeSet) ([]pluginsdk.Asset, []pluginsdk.Statistic) {
	displayNames := make([]string, 0, len(items))
	for _, item := range items {
		displayNames = append(displayNames, item.resource.DisplayName)
	}
	naming := newNames(displayNames)

	assets := make([]pluginsdk.Asset, 0, len(items))
	var statistics []pluginsdk.Statistic

	for _, item := range items {
		dataset := item.resource
		id := s.locate(dataset.Name, item.location)

		name := naming.resolve(dataset.DisplayName, id.id)
		if name == "" {
			log.Warn().Str("resource_name", dataset.Name).Msg("Skipping a dataset with no display name or id")
			continue
		}

		input := datasetSources(dataset.Metadata)
		metadata := s.datasetMetadata(dataset, id, input)

		asset := s.newAsset("Dataset", name, dataset.Description, metadata)
		assets = append(assets, asset)

		// A dataset holds pointers to its data, not the data itself, so
		// the count is worth showing even when it is zero.
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.data_item_count",
			Value:      float64(dataset.DataItemCount),
		})

		if input.empty() {
			log.Debug().Str("dataset", name).Str("metadata_schema_uri", dataset.MetadataSchemaUri).
				Msg("Dataset metadata names no source this plugin recognises")
		}

		// Both ends of these edges belong to other plugins, so the server
		// drops the ones with nothing behind them.
		for _, uri := range input.bigQueryURIs {
			if table := bigQueryTable(uri); table != "" {
				edges.add(mrn.New("Table", "BigQuery", table), *asset.MRN, "FEEDS")
			}
		}
		for _, uri := range input.gcsURIs {
			if bucket := gcsBucket(uri); bucket != "" {
				edges.add(mrn.New("Bucket", "GCS", bucket), *asset.MRN, "FEEDS")
			}
		}
		for _, bucket := range input.buckets {
			edges.add(mrn.New("Bucket", "GCS", bucket), *asset.MRN, "FEEDS")
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered datasets")
	return assets, statistics
}

func (s *Source) datasetMetadata(dataset *aiplatform.GoogleCloudAiplatformV1Dataset, id resourceName, input datasetInput) map[string]any {
	metadata := commonMetadata(id, dataset.CreateTime, dataset.UpdateTime, dataset.Labels)

	putString(metadata, "display_name", dataset.DisplayName)
	putString(metadata, "metadata_schema_uri", dataset.MetadataSchemaUri)
	metadata["data_item_count"] = dataset.DataItemCount

	kind := datasetKind(dataset.MetadataSchemaUri)
	if kind == "" && dataset.MetadataSchemaUri != "" {
		log.Warn().Str("dataset", dataset.Name).Str("metadata_schema_uri", dataset.MetadataSchemaUri).
			Msg("Metadata schema URI does not name a dataset kind")
	}
	putString(metadata, "dataset_kind", kind)

	metadata["saved_query_count"] = len(dataset.SavedQueries)
	putString(metadata, "model_reference", dataset.ModelReference)
	putString(metadata, "source_uris", strings.Join(input.uris(), ", "))

	return metadata
}

// datasetKind names the schema a managed dataset follows, for example
// image_1.0.0, taken from the last segment of its metadata schema URI.
func datasetKind(uri string) string {
	kind, ok := strings.CutSuffix(resourceID(uri), ".yaml")
	if !ok {
		return ""
	}
	return kind
}

// datasetInput is the data a managed dataset points at.
type datasetInput struct {
	bigQueryURIs []string
	gcsURIs      []string
	// buckets are named without a scheme by the image, text and video
	// dataset schemas.
	buckets []string
}

func (d datasetInput) empty() bool {
	return len(d.bigQueryURIs) == 0 && len(d.gcsURIs) == 0 && len(d.buckets) == 0
}

// uris lists the source URIs in a stable order, for the metadata field.
func (d datasetInput) uris() []string {
	uris := append(append([]string{}, d.bigQueryURIs...), d.gcsURIs...)
	sort.Strings(uris)
	return uris
}

// datasetSources reads what a managed dataset points at out of its
// metadata. Dataset.Metadata is untyped and its shape comes from the
// dataset's own metadata schema, so this only reads the shapes Google
// publishes under gs://google-cloud-aiplatform/schema/dataset/metadata and
// ignores everything else. A wrong edge is worse than a missing one.
func datasetSources(metadata any) datasetInput {
	root, ok := metadata.(map[string]any)
	if !ok {
		return datasetInput{}
	}

	var input datasetInput

	// The image, text and video schemas name the bucket holding their
	// blobs directly, with no scheme.
	if bucket, ok := root["gcsBucket"].(string); ok && bucket != "" {
		input.buckets = append(input.buckets, bucket)
	}

	config, ok := root["inputConfig"].(map[string]any)
	if !ok {
		return input
	}

	// The tabular schemas put the source under "uri", a single string for
	// BigQuery and a list for Cloud Storage. The time series schema names
	// the two separately. Every value is classified by its scheme rather
	// than by the config's own type field, so an unexpected combination
	// cannot produce an edge to the wrong technology.
	for _, key := range []string{"uri", "gcs_uri", "bigquery_uri"} {
		for _, uri := range stringsAt(config, key) {
			switch {
			case strings.HasPrefix(uri, "bq://"):
				input.bigQueryURIs = append(input.bigQueryURIs, uri)
			case strings.HasPrefix(uri, "gs://"):
				input.gcsURIs = append(input.gcsURIs, uri)
			}
		}
	}

	return input
}

// stringsAt reads a key that holds either one string or a list of them,
// which is how the dataset schemas spell a single source and several.
func stringsAt(m map[string]any, key string) []string {
	switch value := m[key].(type) {
	case string:
		if value == "" {
			return nil
		}
		return []string{value}
	case []any:
		var out []string
		for _, entry := range value {
			if s, ok := entry.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
