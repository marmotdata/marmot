---
title: Google Cloud Storage
description: Discovers buckets from Google Cloud Storage.
status: experimental
---

# Google Cloud Storage

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


The Google Cloud Storage plugin discovers buckets from GCP projects. It captures bucket metadata including location, storage class, encryption settings, and lifecycle rules.

## Connection Examples

import { Collapsible } from "@site/src/components/Collapsible";

<Collapsible title="Service Account File" icon="logos:google-cloud">

```yaml
project_id: "my-gcp-project"
credentials_file: "/path/to/service-account.json"
include_metadata: true
tags:
  - "gcs"
  - "storage"
```

</Collapsible>

<Collapsible title="Service Account JSON" icon="mdi:key">

```yaml
project_id: "my-gcp-project"
credentials_json: "${GCS_CREDENTIALS_JSON}"
include_metadata: true
include_object_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "gcs"
```

</Collapsible>

## Required Permissions

The service account needs the following IAM roles:

- **Storage Object Viewer** (`roles/storage.objectViewer`) - For listing buckets and objects

Or use a custom role with these permissions:
- `storage.buckets.list`
- `storage.buckets.get`
- `storage.objects.list` (if using object count)



## Example Configuration

```yaml

project_id: "my-gcp-project"
credentials_file: "/path/to/service-account.json"
include_metadata: true
include_object_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "gcs"
  - "storage"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| credentials_file | string | false | Path to service account JSON file |
| credentials_json | string | false | Service account JSON content |
| disable_auth | bool | false | Disable authentication (for local emulators) |
| endpoint | string | false | Custom endpoint URL (for fake-gcs-server or other emulators) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | plugin.Filter | false | Filter buckets by name pattern |
| include_metadata | bool | false | Include bucket metadata like labels |
| include_object_count | bool | false | Count objects in each bucket (can be slow for large buckets) |
| project_id | string | false | Google Cloud project ID |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| bucket_name | string | Name of the bucket |
| created | string | Bucket creation timestamp |
| encryption | string | Encryption type (google-managed or customer-managed) |
| kms_key | string | Customer-managed encryption key name |
| lifecycle_rules_count | int | Number of lifecycle rules configured |
| location | string | Geographic location of the bucket |
| location_type | string | Location type (region, dual-region, multi-region) |
| logging_enabled | bool | Whether access logging is enabled |
| object_count | int64 | Number of objects in the bucket |
| requester_pays | bool | Whether requester pays for access |
| retention_period_seconds | int64 | Retention period in seconds |
| storage_class | string | Default storage class (STANDARD, NEARLINE, COLDLINE, ARCHIVE) |
| versioning | string | Whether object versioning is enabled |