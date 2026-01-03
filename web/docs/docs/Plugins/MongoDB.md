---
title: MongoDB
description: This plugin discovers databases and collections from MongoDB instances.
status: experimental
---

# MongoDB

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>


The MongoDB plugin discovers databases and collections from MongoDB instances. It samples documents to infer schema and captures index information.

## Required Permissions

The user needs read access to discover collections:

```javascript
db.createUser({
  user: "marmot_reader",
  pwd: "your-password",
  roles: [{ role: "read", db: "your_database" }]
})
```

For discovering all databases, use the `readAnyDatabase` role.


## Example Configuration

```yaml

host: "mongo-cluster.company.com"
port: 27017
user: "analytics_reader"
password: "mongo_pass_456"
auth_source: "admin"
tls: true
tags:
  - "mongodb"
  - "analytics"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| auth_source | string | false | Authentication database name |
| collection_filter | plugin.Filter | false | Filter configuration for collections |
| connection_uri | string | false | MongoDB connection URI (overrides host/port/user/password) |
| database_filter | plugin.Filter | false | Filter configuration for databases |
| exclude_system_dbs | bool | false | Whether to exclude system databases (admin, config, local) |
| external_links | []ExternalLink | false | External links to show on all assets |
| host | string | false | MongoDB server hostname or IP address |
| include_collections | bool | false | Whether to discover collections |
| include_databases | bool | false | Whether to discover databases |
| include_indexes | bool | false | Whether to include index information |
| include_views | bool | false | Whether to include views |
| password | string | false | Password for authentication |
| port | int | false | MongoDB server port |
| sample_schema | bool | false | Sample documents to infer schema |
| sample_size | int | false | Number of documents to sample (-1 for entire collection) |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | bool | false | Enable TLS/SSL for connection |
| tls_insecure | bool | false | Skip verification of server certificate |
| use_random_sampling | bool | false | Use random sampling for schema inference |
| user | string | false | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| background | bool | Whether the index was built in the background |
| capped | bool | Whether the collection is capped |
| collection | string | Collection name |
| created | string | Creation timestamp if available |
| data_types | []string | Observed data types |
| database | string | Database name |
| description | string | Field description from validation schema if available |
| document_count | int64 | Approximate document count |
| field_name | string | Field name |
| fields | string | Fields included in the index |
| frequency | float64 | Frequency of field occurrence in documents |
| host | string | MongoDB server hostname |
| index_count | int | Number of indexes on collection |
| is_required | bool | Whether field appears in all documents |
| max_documents | int64 | Maximum document count for capped collections |
| max_size | int64 | Maximum size for capped collections |
| name | string | Index name |
| object_type | string | Object type (collection, view) |
| partial | bool | Whether the index is partial |
| partial_filter | string | Filter expression for partial indexes |
| port | int | MongoDB server port |
| replicated | bool | Whether collection is replicated |
| sample_values | string | Sample values from documents |
| shard_key | string | Shard key if collection is sharded |
| sharding_enabled | bool | Whether sharding is enabled |
| size | int64 | Collection size in bytes |
| sparse | bool | Whether the index is sparse |
| storage_engine | string | Storage engine used |
| ttl | int | Time-to-live in seconds if TTL index |
| type | string | Index type (e.g., single field, compound, text, geo) |
| unique | bool | Whether the index enforces uniqueness |
| validation_action | string | Validation action if schema validation is enabled |
| validation_level | string | Validation level if schema validation is enabled |