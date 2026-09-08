---
title: Couchbase
description: Discovers buckets, scopes and collections from Couchbase Server and Capella clusters.
status: experimental
---

# Couchbase

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


The Couchbase plugin discovers buckets, scopes and collections from Couchbase Server and Capella clusters. Each bucket becomes a `Bucket` asset and each collection a `Collection` asset named `bucket.scope.collection`, linked by `CONTAINS` lineage. The plugin infers a document schema by sampling each collection, records query indexes and collects document counts and bucket sizes.

## Required Permissions

Discovery needs a user that can list buckets, scopes and collections, run read-only N1QL queries (`SELECT`, `INFER`, `system:indexes`) and read bucket statistics from the management API on port 8091 (18091 for `couchbases://`).

On Community Edition, `bucket_full_access` on the buckets to catalog is enough:

```
couchbase-cli user-manage -c localhost -u Administrator -p password --set \
  --rbac-username marmot_reader --rbac-password your-password \
  --roles 'bucket_full_access[*]' --auth-domain local
```

On Enterprise Edition and Capella, grant `query_select` on those buckets, `query_system_catalog` for the index listing, and a role that can read bucket settings and statistics, such as `ro_admin`.

## Indexes and Sampling

Sampling reads documents through the query service, which needs a primary index or, on Couchbase Server 7.6 and later, a sequential scan. Collections without one fall back to `INFER`, which samples through the data service and needs no index. Document counts are skipped when no index can answer them; on a bucket with a single collection the bucket's item count stands in. Data previews always need a primary index on the collection.



## Example Configuration

```yaml

connection_string: "couchbases://cb.abc123.cloud.couchbase.com"
username: "marmot_reader"
password: "couchbase_pass_789"
bucket: "travel-sample"
include_columns: true
sample_size: 100
include_indexes: true
include_statistics: true
tags:
  - "couchbase"
  - "travel"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| bucket | string | false | Only discover this bucket (all buckets when empty) |
| connect_timeout_seconds | int | false | Seconds to wait for the cluster connection |
| connection_string | string | true | Cluster connection string: couchbase://host, or couchbases://host for TLS (Capella) |
| exclude_buckets | []string | false | Bucket names to skip |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_columns | bool | false | Whether to infer a document schema by sampling each collection |
| include_indexes | bool | false | Whether to include query index information |
| include_statistics | bool | false | Whether to collect document counts and bucket sizes |
| include_system_scopes | bool | false | Whether to include the _system scope |
| password | string | true | Password for authentication |
| sample_size | int | false | Number of documents to sample per collection for schema inference |
| ssl_skip_verify | bool | false | Skip TLS certificate verification for couchbases:// connections |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | true | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| bucket | string | Bucket name |
| bucket_type | string | Bucket type (couchbase, ephemeral, memcached) |
| cluster_version | string | Couchbase Server version |
| collection | string | Collection name |
| collection_count | int | Number of discovered collections |
| column_name | string | Field name, with nested fields as parent.child |
| conflict_resolution | string | XDCR conflict resolution type (seqno, lww, custom) |
| data_type | string | Observed JSON type, joined with \| when mixed |
| data_used_bytes | int64 | Size of the bucket's data in bytes |
| disk_used_bytes | int64 | Disk space used by the bucket in bytes |
| document_count | int64 | Number of documents in the collection |
| durability_min_level | string | Minimum durability level for writes |
| eviction_policy | string | Eviction policy (valueOnly, fullEviction, noEviction, nruEviction) |
| flush_enabled | bool | Whether the bucket can be flushed |
| history | bool | Whether change history retention is enabled |
| index_count | int | Number of query indexes on the collection |
| indexes | []string | Query index names |
| is_nullable | bool | Whether the field is absent from some sampled documents |
| item_count | int64 | Number of documents in the bucket |
| max_ttl | int64 | Maximum document expiry in seconds, when set |
| mem_used_bytes | int64 | Memory used by the bucket in bytes |
| occurrence | float64 | Fraction of sampled documents holding the field |
| primary_index | bool | Whether the collection has a primary index |
| ram_quota_mb | int64 | Memory quota per node in MB |
| replicas | int | Number of replica copies |
| scope | string | Scope name |
| scope_count | int | Number of discovered scopes |
| storage_backend | string | Storage backend (couchstore, magma) |
