---
title: MLflow
description: Discovers registered models, experiments and training datasets from MLflow tracking servers.
status: experimental
---

# MLflow

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
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


The MLflow plugin discovers registered models, experiments and training datasets from an MLflow tracking server. Every registered model becomes a Model asset carrying the run behind its newest version: hyperparameters, latest metric values, the experiment it came from and the input features of its signature. Experiments become Experiment assets and the datasets logged to a model's run become Dataset assets.

Lineage links each Experiment to the models it produced (PRODUCES) and each Dataset to the models trained on it (FEEDS). A dataset read from `s3://` or `gs://` is also linked to the bucket asset the S3 or GCS plugin creates. Run metrics are recorded as `asset.metric.<name>` statistics.

## Authentication

The tracking server is contacted anonymously unless `username` and `password` (MLflow's basic auth) or `token` (a bearer token, for servers behind a proxy) are set. Set one or the other, not both.

## Model Signatures

Features are read from the run's `mlflow.log-model.history` tag. When the run has none, the plugin reads the MLmodel file of the logged model (MLflow 3, `models:/` sources) or of the run's artifacts, which needs a tracking server that stores or proxies its own artifacts (`--serve-artifacts`). A model whose signature cannot be found is still discovered, without features.



## Example Configuration

```yaml

tracking_uri: "https://mlflow.company.com"
username: "marmot"
password: "mlflow_secure_pass"
include_experiments: true
include_datasets: true
include_metrics: true
tags:
  - "mlflow"
  - "ml-platform"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_datasets | bool | false | Discover the datasets logged to each model's run |
| include_experiments | bool | false | Discover experiments as assets |
| include_metrics | bool | false | Record the run metrics of each model |
| max_models | int | false | Maximum number of registered models to discover (0 = unlimited) |
| password | string | false | Password for basic authentication |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token for authentication |
| tracking_uri | string | true | MLflow tracking server URL, also used as the model registry |
| username | string | false | Username for basic authentication |
| verify_ssl | bool | false | Verify the server TLS certificate |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| aliases | map[string]string | Alias to version number |
| artifact_location | string | Where the experiment's runs store artifacts |
| artifact_uri | string | Where the run's artifacts are stored |
| column_name | string | Feature or column name |
| context | string | What the dataset was used for (training, eval) |
| created_at | string | When the model or experiment was created |
| data_type | string | MLflow data type |
| description | string | Registered model description |
| digest | string | Content digest MLflow computed for the dataset |
| experiment | string | Name of the experiment the run belongs to |
| experiment_id | string | Experiment id |
| hyperparameters | map[string]string | Parameters logged to the run |
| is_nullable | bool | Whether the input is optional |
| latest_version | string | Highest version number |
| lifecycle_stage | string | Lifecycle stage (active) |
| metrics | map[string]any | Latest value of each metric logged to the run |
| profile | any | Profile MLflow computed for the dataset, such as row counts |
| run_id | string | Run that produced the latest version |
| run_name | string | Name of that run |
| source | any | Where the dataset was read from |
| source_type | string | Kind of source the dataset was read from |
| stage | string | Stage of the latest version, when one is set |
| status | string | Status of the latest version |
| tags | map[string]string | Registered model or experiment tags |
| updated_at | string | When the model or experiment was last updated |
| url | string | Link to the model or experiment in the MLflow UI |
| version_count | int | Number of versions |
