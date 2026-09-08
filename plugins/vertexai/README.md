The Vertex AI plugin discovers models, prediction endpoints, managed datasets, feature groups and pipeline jobs from a Google Cloud project.

Managed datasets and feature groups are both catalogued as Datasets, with a feature group's features as the schema. Pipeline jobs are catalogued as Jobs with run history.

Pipeline jobs are off by default because a project keeps a long job history. Turn them on with `include_pipeline_jobs: true`.

Vertex AI has no wildcard location and serves regional resources from the location's own host, so `locations` is required and each one is scanned through its own endpoint.

## Naming

Vertex AI display names are not unique. An asset is named after its display name, and when more than one resource of the same kind shares one, every one of them is named `display name (resource id)`. A feature group is named after its id.

## Lineage

| Edge | Meaning |
|------|---------|
| GCS bucket FEEDS model | The bucket holding the model artifact |
| Model FEEDS endpoint | The endpoint serves the model |
| Pipeline job PRODUCES model | The job the model was produced by |
| BigQuery table FEEDS dataset | The table a tabular or time series dataset reads |
| GCS bucket FEEDS dataset | The bucket a dataset reads its files or blobs from |
| BigQuery table FEEDS feature group | The table the feature group reads its features from |

Edges into BigQuery and Cloud Storage name assets those plugins own. Marmot drops an edge whose other end is not catalogued.

## Required Permissions

`roles/aiplatform.viewer` covers every call this plugin makes. With no credentials set, Application Default Credentials are used.

## Testing

Google publishes no Vertex AI emulator. The end to end tests serve the aiplatform API themselves, built from the generated API structs, and drive the plugin binary over the gRPC wire:

    MARMOT_TEST_VERTEXAI_ENDPOINT=http://127.0.0.1:18821 go test ./...
