package mlflow

import (
	"encoding/json"
	"net/url"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// dataContextTag is the run input tag that says what the dataset was used
// for, typically "training" or "eval".
const dataContextTag = "mlflow.data.context"

// datasetSchema is the schema document MLflow logs for a tabular dataset.
type datasetSchema struct {
	Columns []signatureInput `json:"mlflow_colspec"`
}

// addDataset emits the Dataset asset behind one run input, or folds the
// input into the asset already emitted for the same dataset name, and
// returns the asset's MRN. An input without a name cannot be an asset and
// returns "".
func (d *discovery) addDataset(input datasetInput) string {
	ds := input.Dataset
	if ds.Name == "" {
		return ""
	}

	context := ""
	for _, t := range input.Tags {
		if t.Key == dataContextTag {
			context = t.Value
		}
	}

	if i, seen := d.datasets[ds.Name]; seen {
		mergeContext(&d.assets[i], context)
		return *d.assets[i].MRN
	}

	metadata := map[string]any{}
	putIf(metadata, "digest", ds.Digest)
	putIf(metadata, "source_type", ds.SourceType)
	putIf(metadata, "context", context)
	if source := jsonOrString(ds.Source); source != nil {
		metadata["source"] = source
	}
	if profile := jsonOrString(ds.Profile); profile != nil {
		metadata["profile"] = profile
	}

	asset := d.newAsset("Dataset", ds.Name, metadata)
	if columns := datasetColumns(ds); len(columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, columns); err != nil {
			log.Warn().Err(err).Str("dataset", ds.Name).Msg("Failed to set dataset columns")
		}
	}

	d.datasets[ds.Name] = len(d.assets)
	d.assets = append(d.assets, asset)

	if bucketMRN := bucketMRN(datasetURI(ds.Source)); bucketMRN != "" {
		d.link(bucketMRN, *asset.MRN, "FEEDS")
	}

	return *asset.MRN
}

// mergeContext records a further use of an already emitted dataset, so a
// dataset used for both training and evaluation reads "eval, training"
// rather than whichever input came first.
func mergeContext(asset *pluginsdk.Asset, context string) {
	if context == "" {
		return
	}
	seen := make(map[string]struct{})
	if existing, ok := asset.Metadata["context"].(string); ok && existing != "" {
		for _, c := range strings.Split(existing, ", ") {
			seen[c] = struct{}{}
		}
	}
	seen[context] = struct{}{}
	asset.Metadata["context"] = strings.Join(sortedKeys(seen), ", ")
}

// datasetColumns reads the column list MLflow logs for tabular datasets.
// Tensor datasets carry a different document and yield no columns.
func datasetColumns(ds dataset) []pluginsdk.Column {
	if ds.Schema == "" {
		return nil
	}
	var schema datasetSchema
	if err := json.Unmarshal([]byte(ds.Schema), &schema); err != nil {
		log.Debug().Err(err).Str("dataset", ds.Name).Msg("Could not parse the dataset schema")
		return nil
	}
	return featureColumns(schema.Columns)
}

// jsonOrString decodes a value MLflow stores as a JSON string, keeping it
// verbatim when it is not JSON. Empty values yield nil so they stay out
// of the metadata.
func jsonOrString(value string) any {
	if value == "" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return value
	}
	return decoded
}

// datasetURI returns the location a dataset source points at. MLflow
// records sources as a JSON object whose shape depends on the source
// type; the ones that point at storage carry a uri or a path.
func datasetURI(source string) string {
	decoded, ok := jsonOrString(source).(map[string]any)
	if !ok {
		return strings.TrimSpace(source)
	}
	for _, key := range []string{"uri", "path", "url"} {
		if s, ok := decoded[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// bucketMRN returns the MRN Marmot's S3 or GCS plugin gives the bucket a
// URI lives in, so a dataset read from object storage links to the bucket
// asset those plugins create. Any other location yields "".
func bucketMRN(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Host == "" {
		return ""
	}
	switch u.Scheme {
	case "s3", "s3a", "s3n":
		return assetMRNFor("Bucket", "S3", u.Host)
	case "gs":
		return assetMRNFor("Bucket", "GCS", u.Host)
	default:
		return ""
	}
}
