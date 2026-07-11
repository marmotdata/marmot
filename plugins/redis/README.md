---
title: Redis
description: Discovers databases from Redis instances.
status: experimental
---

# Redis

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span></div>
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


The Redis plugin discovers logical databases (db0–db15) from Redis instances. It uses the `INFO` command to collect server metadata and parses the Keyspace section to identify databases that contain keys.

## Required Permissions

The connecting user needs permission to run the `INFO` command. By default all users can run `INFO`, but if you are using Redis ACLs:

```
ACL SETUSER marmot_reader on >password ~* &* +info +ping +select
```



## Example Configuration

```yaml

host: "localhost"
port: 6379
password: "secret"
discover_all_databases: true
filter:
  include:
    - "^db[0-3]$"
tags:
  - "redis"
  - "cache"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| db | int | false | Default database number |
| discover_all_databases | bool | false | Discover all databases with keys (db0-db15) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | false | Redis server hostname or IP address |
| password | string | false | Password for authentication |
| port | int | false | Redis server port |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | bool | false | Enable TLS connection |
| tls_insecure | bool | false | Skip TLS certificate verification |
| username | string | false | Username for ACL authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| avg_ttl_ms | int64 | Average TTL in milliseconds |
| connected_clients | string | Number of connected clients |
| database | string | Database name (e.g. db0) |
| expires_count | int64 | Number of keys with an expiration |
| host | string | Redis server hostname |
| key_count | int64 | Number of keys in the database |
| maxmemory_policy | string | Eviction policy when maxmemory is reached |
| port | int | Redis server port |
| redis_version | string | Redis server version |
| role | string | Replication role (master/slave) |
| uptime_seconds | string | Server uptime in seconds |
| used_memory_human | string | Human-readable used memory |