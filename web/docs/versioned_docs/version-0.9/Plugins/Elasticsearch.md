---
title: Elasticsearch
description: This plugin discovers indices, data streams, and aliases from Elasticsearch clusters.
status: experimental
---

# Elasticsearch

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
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The Elasticsearch plugin discovers indices, data streams and aliases from Elasticsearch clusters.

## Required Permissions

The connecting user needs `monitor` cluster privilege and `view_index_metadata` on indices. The built-in `viewer` role is usually sufficient.



## Example Configuration

```yaml

addresses:
  - "https://elasticsearch.company.com:9200"
username: "elastic"
password: "changeme"
tags:
  - "elasticsearch"
  - "search"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| addresses | []string | false | List of Elasticsearch node URLs |
| api_key | string | false | API key for authentication (mutually exclusive with username/password) |
| ca_cert_path | string | false | Path to a custom CA certificate file |
| cloud_id | string | false | Elastic Cloud ID for connecting to Elastic Cloud |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_aliases | bool | false | Discover aliases |
| include_data_streams | bool | false | Discover data streams |
| include_index_stats | bool | false | Collect document count and store size metrics |
| include_system_indices | bool | false | Include system indices (prefixed with .) |
| password | string | false | Password for basic authentication |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls_skip_verify | bool | false | Skip TLS certificate verification |
| username | string | false | Username for basic authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| alias_name | string | Name of the alias |
| analyzer | string | Analyzer used for the field |
| backing_indices | int | Number of backing indices |
| cluster | string | Name of the Elasticsearch cluster |
| creation_date | string | Date and time when the index was created |
| data_stream_name | string | Name of the data stream |
| docs_count | int64 | Number of documents in the index |
| field_name | string | Full dotted path of the field |
| field_type | string | Elasticsearch field type (keyword, text, long, etc.) |
| filter_defined | string | Whether a filter is defined on the alias |
| generation | int | Current generation of the data stream |
| health | string | Health status of the index (green, yellow, red) |
| ilm_policy | string | ILM policy applied to the data stream |
| index | string | Whether the field is indexed |
| index_name | string | Name of the index |
| indices | string | Comma-separated list of indices the alias points to |
| is_write_index | string | Whether the alias has a designated write index |
| replicas | int | Number of replica shards |
| shards | int | Number of primary shards |
| status | string | Health status of the data stream |
| status | string | Open/close status of the index |
| store_size | string | Total store size of the index |
| template | string | Index template used by the data stream |
| timestamp_field | string | Name of the timestamp field |
| uuid | string | UUID of the index |