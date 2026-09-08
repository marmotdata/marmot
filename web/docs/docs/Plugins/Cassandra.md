---
title: Cassandra
description: Discovers keyspaces, tables and materialized views from Apache Cassandra clusters.
status: experimental
---

# Cassandra

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


The Cassandra plugin discovers keyspaces, tables and materialized views from Apache Cassandra clusters. It reads the `system_schema` tables over CQL with the pure-Go `gocql` driver, so it works with Cassandra 3.0 and later.

Each keyspace becomes a Keyspace asset linked to its tables and views by CONTAINS edges, and each materialized view is linked to its base table by a VIEW_OF edge. Tables and views are named `keyspace.table`, the same identity the OpenMetadata plugin projects for a Cassandra service, so both routes land on one asset.

Columns keep the CQL type exactly as Cassandra stores it (`map<text, int>`, `frozen<address>`) and record whether they are part of the partition key, a clustering column, a static column or a regular one. A materialized view carries its definition as a CQL query rebuilt from the base table, selected columns and filter.

## Statistics

Column and index counts are reported per table. A row count is not: Cassandra keeps no row estimate and `COUNT(*)` is a full cluster scan.

## Known Gaps

DataStax Astra secure connect bundles are not supported. Connect with `hosts` and `ssl` instead.



## Example Configuration

```yaml

hosts:
  - "cassandra-1.internal:9042"
  - "cassandra-2.internal"
port: 9042
username: "marmot_reader"
password: "cassandra_secure_pass"
datacenter: "dc1"
keyspaces:
  - "shop"
include_columns: true
include_views: true
include_indexes: true
include_statistics: true
filter:
  include:
    - "^shop.*"
  exclude:
    - ".*_tmp$"
tags:
  - "cassandra"
  - "shop"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| connect_timeout_seconds | int | false | Seconds to wait for a connection |
| datacenter | string | false | Local datacenter; only its nodes are queried when set |
| exclude_system_keyspaces | bool | false | Whether to skip Cassandra's own keyspaces (system, system_schema, system_auth, ...) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| hosts | []string | true | Contact points, as host or host:port |
| include_columns | bool | false | Whether to include column information in table metadata |
| include_indexes | bool | false | Whether to include secondary index information |
| include_statistics | bool | false | Whether to include column and index counts |
| include_views | bool | false | Whether to include materialized views |
| keyspaces | []string | false | Keyspaces to discover; all non-system keyspaces when empty |
| password | string | false | Password for password authentication |
| port | int | false | Port for contact points given without one |
| ssl | bool | false | Connect over TLS |
| ssl_ca_cert | string | false | Path to the CA certificate that signed the server certificate |
| ssl_skip_verify | bool | false | Skip verification of the server certificate |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | false | Username for password authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| base_table | string | Table a materialized view is built from |
| bloom_filter_fp_chance | float64 | Bloom filter false positive chance |
| caching | map[string]string | Key and row cache settings |
| cassandra_version | string | Cassandra release version of the node queried |
| cluster_name | string | Cluster name |
| clustering_columns | []string | Clustering columns in key order |
| clustering_order | map[string]string | Sort direction (asc, desc) per clustering column |
| column_name | string | Column name |
| comment | string | Table comment |
| compaction_class | string | Compaction strategy |
| compression_class | string | Compressor |
| data_type | string | CQL type as stored, for example map\<text, int> or frozen\<address> |
| datacenter | string | Datacenter of the node queried |
| default_ttl | int | Default time to live in seconds, 0 for none |
| durable_writes | bool | Whether writes go through the commit log |
| flags | []string | Table flags (compound, counter, dense, super) |
| gc_grace_seconds | int | Seconds tombstones are kept before collection |
| id | string | Table id |
| include_all_columns | bool | Whether a materialized view selects every base table column |
| index_count | int | Number of secondary indexes |
| indexes | []string | Secondary index names |
| is_counter | bool | Whether the table holds counter columns |
| is_nullable | bool | Whether the column can be null (false for key columns) |
| is_primary_key | bool | Whether the column is part of the partition or clustering key |
| keyspace | string | Keyspace name |
| kind | string | Column kind (partition_key, clustering, regular, static) |
| object_type | string | Object type (table, view) |
| partition_key | []string | Partition key columns in key order |
| position | int | Index within the partition or clustering key, -1 otherwise |
| replication_class | string | Replication strategy (SimpleStrategy, NetworkTopologyStrategy) |
| replication_factor | int | Replication factor when one applies to the whole keyspace |
| replication_factors | map[string]int | Replication factor per datacenter |
| table_count | int | Number of tables in the keyspace |
| table_name | string | Table or view name |
| user_types | []string | User-defined types declared in the keyspace |
| view_count | int | Number of materialized views in the keyspace |
| where_clause | string | Filter a materialized view applies to its base table |
