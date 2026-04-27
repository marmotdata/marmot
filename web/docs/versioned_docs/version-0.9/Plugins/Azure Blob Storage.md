---
title: Azure Blob Storage
description: Discovers containers and blobs from Azure Blob Storage accounts.
status: experimental
---

# Azure Blob Storage

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


The Azure Blob Storage plugin discovers containers from Azure Storage accounts. It captures container metadata including access levels, lease status, and custom metadata.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Connection String" icon="logos:microsoft-azure">

```yaml
connection_string: "${AZURE_STORAGE_CONNECTION_STRING}"
include_metadata: true
tags:
  - "azure"
  - "storage"
```

</Collapsible>

<Collapsible title="Account Name and Key" icon="mdi:key">

```yaml
account_name: "mystorageaccount"
account_key: "${AZURE_STORAGE_ACCOUNT_KEY}"
include_metadata: true
include_blob_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "azure"
```

</Collapsible>

## Required Permissions

The following Azure RBAC role is recommended:

- **Storage Blob Data Reader** - Read access to containers and blobs

Or use a custom role with these permissions:

- `Microsoft.Storage/storageAccounts/blobServices/containers/read`
- `Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read`



## Example Configuration

```yaml

connection_string: "${AZURE_STORAGE_CONNECTION_STRING}"
include_metadata: true
include_blob_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "azure"
  - "storage"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| account_key | string | false | Azure Storage account key |
| account_name | string | false | Azure Storage account name |
| connection_string | string | false | Azure Storage connection string |
| endpoint | string | false | Custom endpoint URL (for Azurite or other emulators) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| include_blob_count | bool | false | Count blobs in each container (can be slow for large containers) |
| include_metadata | bool | false | Include container metadata |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| blob_count | int64 | Number of blobs in the container |
| container_name | string | Name of the container |
| etag | string | Entity tag for the container |
| has_immutability_policy | bool | Whether container has an immutability policy |
| has_legal_hold | bool | Whether container has a legal hold |
| last_modified | string | Last modification timestamp |
| lease_state | string | Lease state (available/leased/expired/breaking/broken) |
| lease_status | string | Lease status (locked/unlocked) |
| public_access | string | Public access level (none/blob/container) |