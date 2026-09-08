---
title: Vertex AI
description: Discovers models, endpoints, datasets, feature groups and pipeline jobs from Google Vertex AI.
status: experimental
---

# Vertex AI

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Vertex AI plugin discovers models, prediction endpoints, managed datasets, feature groups and pipeline jobs from a Google Cloud project.

Managed datasets and feature groups are both catalogued as Datasets, with a feature group's features as the schema. Pipeline jobs are catalogued as Jobs with run history.

Pipeline jobs are off by default because a project keeps a long job history. Turn them on with `include_pipeline_jobs: true` and set `max_pipeline_jobs` to how many recent runs you want.

## Locations

Vertex AI has no wildcard location, and it serves regional resources from the location's own host rather than from the API's default one. List every region you use under `locations`; each one is scanned through its own endpoint.

## Naming

Vertex AI display names are not unique, and the unique id is a meaningless number. An asset is named after its display name. When more than one resource of the same kind shares a display name, every one of them is named `display name (resource id)`, so a name never depends on the order the API listed things in. The id is always in the `resource_id` metadata field.

A feature group is named after its id, which the user chooses and Vertex AI keeps unique within a project and location.

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

The service account needs read access to Vertex AI. The `roles/aiplatform.viewer` role covers every call this plugin makes: listing models, endpoints, datasets, feature groups, features and pipeline jobs.

## Example Configuration

```yaml

project_id: "acme-ml"
locations:
  - "us-central1"
  - "europe-west4"
credentials_file: "/etc/marmot/vertexai.json"
include_endpoints: true
include_datasets: true
include_feature_groups: true
include_pipeline_jobs: true
max_pipeline_jobs: 100
tags:
  - "gcp"
  - "ml"

```

With no credentials set, Application Default Credentials are used.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to service account JSON file |
| credentials_json | string | false | Service account JSON content |
| disable_auth | bool | false | Disable authentication, for local testing |
| endpoint | string | false | Custom endpoint URL, for testing against a local server |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_datasets | bool | false | Whether to discover managed datasets |
| include_endpoints | bool | false | Whether to discover prediction endpoints |
| include_feature_groups | bool | false | Whether to discover feature groups from the feature store |
| include_pipeline_jobs | bool | false | Whether to discover pipeline jobs. Projects keep a long job history, so this is off by default |
| locations | []string | true | Regions to scan |
| max_pipeline_jobs | int | false | How many recent pipeline jobs to read |
| project_id | string | true | Google Cloud project ID |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

Every asset carries the common fields. A boolean field is only set when it is true, because the API omits a false one.

| Field | Type | Description |
|-------|------|-------------|
| artifact_uri | string | Cloud Storage directory holding the model artifact |
| base_model_source | string | Model Garden name or Genie URI of the model this one is derived from |
| big_query_source_uri | string | BigQuery table a feature group reads its features from |
| column_name | string | Feature id, in a feature group's schema |
| container_image | string | Container image the model is served from |
| create_time | string | When the resource was created |
| data_item_count | int | Number of data items in a managed dataset |
| data_type | string | Feature value type, lowercased, in a feature group's schema |
| dataset_kind | string | Kind of dataset, read from the metadata schema URI, for example image_1.0.0 |
| dedicated_endpoint_dns | string | DNS name of the dedicated endpoint |
| dedicated_endpoint_enabled | bool | Set when the endpoint has a dedicated DNS name |
| dense | bool | Set when a feature group writes every feature on every row |
| deployed_model_count | int | Number of models deployed to an endpoint, or of endpoints a model is deployed to |
| deployed_models | string | Display names of the models deployed to an endpoint |
| display_name | string | Display name Vertex AI shows, which it does not require to be unique |
| entity_id_columns | []string | Columns of the source table that identify an entity |
| error_message | string | Why a pipeline job failed |
| end_time | string | When a pipeline job finished |
| feature_count | int | Number of features in a feature group, absent when the feature list could not be read |
| feature_group_id | string | Id of the feature group |
| label_&lt;key&gt; | string | One entry per resource label |
| location | string | Region the resource lives in |
| metadata_schema_uri | string | Schema describing a model's or dataset's additional metadata |
| model_deployment_monitoring_job | string | Id of the monitoring job watching an endpoint |
| model_reference | string | Model a dataset was created for |
| network | string | VPC network an endpoint is peered with |
| pipeline_job | string | Id of the pipeline job that produced a model |
| predict_schemata_instance | string | Schema of a single prediction instance |
| predict_schemata_parameters | string | Schema of the prediction parameters |
| predict_schemata_prediction | string | Schema of a single prediction |
| project_id | string | Google Cloud project the resource belongs to |
| resource_id | string | Vertex AI id of the resource, unique within its project and location |
| saved_query_count | int | Number of saved queries defined on a dataset |
| schedule_name | string | Schedule that created a pipeline run |
| service_account | string | Service account a pipeline job ran as |
| service_account_email | string | Service account a feature group reads its source with |
| source_uris | string | BigQuery and Cloud Storage URIs a dataset reads from |
| start_time | string | When a pipeline job started running |
| state | string | Pipeline state, for example PIPELINE_STATE_SUCCEEDED |
| static_data_source | bool | Set when a feature group's source table does not change |
| supported_deployment_resources_types | []string | Resource types the model can be deployed with |
| supported_input_storage_formats | []string | Input formats the model accepts for batch prediction |
| supported_output_storage_formats | []string | Output formats the model writes for batch prediction |
| template_uri | string | Location of the pipeline template a run was compiled from |
| traffic_split | string | How traffic is shared between deployed models, as id=percent pairs |
| training_pipeline | string | Id of the training pipeline that uploaded a model |
| update_time | string | When the resource was last updated |
| version_aliases | []string | Aliases a model version can be referenced by |
| version_create_time | string | When a model version was created |
| version_description | string | Description of a model version |
| version_id | string | Version of the model this entry describes |

## Statistics

| Metric | Asset | Meaning |
|--------|-------|---------|
| asset.data_item_count | Dataset | Number of data items in a managed dataset, emitted even when zero |
| asset.column_count | Dataset | Number of features in a feature group |
