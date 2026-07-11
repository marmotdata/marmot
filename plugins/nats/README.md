---
title: NATS
description: Discovers JetStream streams from NATS servers.
status: experimental
---

# NATS

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


The NATS plugin discovers JetStream streams from NATS servers. It connects using the NATS client protocol and enumerates streams via the JetStream API, collecting configuration and runtime state for each stream.

## Requirements

- **JetStream must be enabled** on the NATS server (start with `-js` flag or configure in `nats-server.conf`). Core NATS subjects are ephemeral and not discoverable as persistent assets.
- The connecting user needs permission to access the JetStream API (`$JS.API.>`).

## Authentication

The plugin supports several authentication methods:

- **Token**: Set the `token` field for token-based auth.
- **Username/Password**: Set `username` and `password` fields.
- **Credentials file**: Set `credentials_file` to the path of a `.creds` file (NKey-based auth).
- **TLS**: Enable `tls` for encrypted connections. Use `tls_insecure` to skip certificate verification in development.



## Example Configuration

```yaml

host: "localhost"
port: 4222
token: "s3cr3t"
filter:
  include:
    - "^ORDERS"
tags:
  - "nats"
  - "messaging"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to NATS credentials file (.creds) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| host | string | false | NATS server hostname or IP address |
| password | string | false | Password for authentication |
| port | int | false | NATS server port |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| tls | bool | false | Enable TLS connection |
| tls_insecure | bool | false | Skip TLS certificate verification |
| token | string | false | Authentication token |
| username | string | false | Username for authentication |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| bytes | uint64 | Total bytes stored in the stream |
| consumer_count | int | Number of consumers attached to the stream |
| discard_policy | string | Policy when limits are reached (Old or New) |
| duplicate_window | string | Duplicate message tracking window |
| first_seq | uint64 | Sequence number of the first message |
| host | string | NATS server hostname |
| last_seq | uint64 | Sequence number of the last message |
| max_age | string | Maximum age of messages |
| max_bytes | int64 | Maximum total bytes for the stream (-1 = unlimited) |
| max_msg_size | int64 | Maximum size of a single message |
| max_msgs | int64 | Maximum number of messages (-1 = unlimited) |
| messages | uint64 | Total number of messages in the stream |
| num_replicas | int | Number of stream replicas |
| port | int | NATS server port |
| retention_policy | string | Message retention policy (Limits, Interest, WorkQueue) |
| storage_type | string | Storage backend (File or Memory) |
| stream_name | string | Name of the JetStream stream |
| subjects | string | Comma-separated list of subjects the stream listens on |