---
title: BigQuery
description: This plugin discovers datasets and tables from Google BigQuery projects.
status: experimental
---

# BigQuery

This plugin discovers datasets and tables from Google BigQuery projects.

**Status:** experimental

## Example Configuration

```yaml

project_id: "company-data-warehouse"
credentials_path: "/etc/marmot/bq-service-account.json"
tags:
  - "bigquery"
  - "data-warehouse"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| aws | AWSConfig | false |  |
| credentials_json | string | false | Service account credentials JSON content |
| credentials_path | string | false | Path to service account credentials JSON file |
| dataset_filter | plugin.Filter | false | Filter configuration for datasets |
| exclude_system_datasets | bool | false | Whether to exclude system datasets (_script, _analytics, etc.) |
| external_links | []ExternalLink | false |  |
| global_documentation | []string | false |  |
| global_documentation_position | string | false |  |
| include_datasets | bool | false | Whether to discover datasets |
| include_external_tables | bool | false | Whether to discover external tables |
| include_table_stats | bool | false | Whether to include table statistics (row count, size) |
| include_views | bool | false | Whether to discover views |
| max_concurrent_requests | int | false | Maximum number of concurrent API requests |
| merge | MergeConfig | false |  |
| metadata | MetadataConfig | false |  |
| project_id | string | false | Google Cloud Project ID |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| tags | TagsConfig | false |  |
| use_default_credentials | bool | false | Use default Google Cloud credentials |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| access_entries_count | int | Number of access control entries |
| clustering_fields | []string | Clustering fields |
| creation_time | string | Dataset creation timestamp |
| creation_time | string | Table creation timestamp |
| dataset_id | string | Dataset ID |
| dataset_id | string | Dataset ID |
| default_partition_expiration | string | Default partition expiration duration |
| default_table_expiration | string | Default table expiration duration |
| description | string | Dataset description |
| description | string | Column description |
| description | string | Table description |
| expiration_time | string | Table expiration timestamp |
| external_data_config | map[string]interface{} | External data configuration for external tables |
| labels | map[string]string | Dataset labels |
| labels | map[string]string | Table labels |
| last_modified | string | Last modification timestamp |
| last_modified | string | Last modification timestamp |
| location | string | Geographic location of the dataset |
| name | string | Column name |
| nested_fields | []map[string]interface{} | Nested fields for RECORD type columns |
| num_bytes | int64 | Size of the table in bytes |
| num_rows | uint64 | Number of rows in the table |
| partition_expiration | string | Partition expiration duration |
| project_id | string | Google Cloud Project ID |
| project_id | string | Google Cloud Project ID |
| range_partitioning_field | string | Range partitioning field |
| source_format | string | Source data format (CSV, JSON, AVRO, etc.) |
| source_uris | []string | Source URIs for external data |
| table_id | string | Table ID |
| table_type | string | Table type (TABLE, VIEW, EXTERNAL) |
| time_partitioning_field | string | Time partitioning field |
| time_partitioning_type | string | Time partitioning type |
| type | string | Column data type |
| view_query | string | SQL query for views |