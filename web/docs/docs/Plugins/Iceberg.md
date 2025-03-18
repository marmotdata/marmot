---
title: Iceberg
description: This plugin discovers Apache Iceberg tables from various catalog implementations.
status: experimental
---

# Iceberg

This plugin discovers Apache Iceberg tables from various catalog implementations.

**Status:** experimental

## Example Configuration

```yaml

catalog_type: "rest"  # Options: "rest", "glue"

# REST catalog configuration
rest:
  uri: "http://localhost:8181"
  auth:
    type: "none"  # Options: "none", "basic", "oauth2", "bearer"
    username: ""
    password: ""
    token: ""
    client_id: ""
    client_secret: ""
    token_url: ""
    cert_path: ""

# AWS Glue catalog configuration
# glue:
#   region: "us-west-2"
#   database: "default"  # Optional: limit discovery to a single database
#   access_key: ""  # Optional: use environment or instance profile if not provided
#   secret_key: ""
#   credentials_profile: ""  # Optional: use named AWS profile
#   assume_role_arn: ""  # Optional: assume role ARN

# Metadata collection options
include_schema_info: true
include_partition_info: true
include_snapshot_info: true
include_properties: true
include_statistics: true

# Filter configuration
table_filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"

namespace_filter:
  include:
    - "^analytics.*"
    - "^data.*"
  exclude:
    - ".*_temp$"

# Common tag configuration
tags:
  - "iceberg"
  - "data-lake"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| catalog_type | string | false | Type of catalog to use: rest, glue |
| rest | RESTConfig | false | REST catalog configuration |
| glue | GlueConfig | false | AWS Glue catalog configuration |
| include_schema_info | bool | false | Whether to include schema information in metadata |
| include_partition_info | bool | false | Whether to include partition information in metadata |
| include_snapshot_info | bool | false | Whether to include snapshot information in metadata |
| include_properties | bool | false | Whether to include table properties in metadata |
| include_statistics | bool | false | Whether to include table statistics in metadata |
| table_filter | plugin.Filter | false | Filter configuration for tables |
| namespace_filter | plugin.Filter | false | Filter configuration for namespaces |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| catalog_name | string | Name of the catalog |
| catalog_type | string | Type of catalog used (rest, hive, s3, adls, local) |
| current_schema_id | int | ID of the current schema |
| current_snapshot_id | int64 | ID of the current snapshot |
| file_size_bytes | int64 | Total size of data files in bytes |
| format_version | int | Iceberg table format version |
| identifier | string | Full identifier of the table (namespace.table_name) |
| last_commit_time | string | Human-readable timestamp when the table was last updated |
| last_updated_ms | int64 | Timestamp when the table was last updated in milliseconds since epoch |
| location | string | Base location URI of the table data |
| maintenance_last_run | int64 | Timestamp of last maintenance run in milliseconds since epoch, if available |
| namespace | string | Namespace of the table |
| num_data_files | int | Number of data files |
| num_delete_files | int | Number of delete files |
| num_partitions | int | Number of partitions |
| num_rows | int64 | Number of rows in the table |
| num_snapshots | int | Number of snapshots in table history |
| orphan_files_size_bytes | int64 | Size of orphan files in bytes, if available |
| partition_spec | string | JSON representation of the partition specification |
| partition_transformers | string | List of partition transformers used (identity, bucket, truncate, etc.) |
| properties | map[string]string | Table properties |
| schema_json | string | JSON representation of the current schema |
| sort_order_json | string | JSON representation of the sort order |
| table_name | string | Name of the table |
| uuid | string | UUID of the table |