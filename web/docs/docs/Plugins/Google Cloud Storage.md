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

## Authentication

import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

The recommended approach is keyless: **[Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials)**. The model is the same everywhere: you grant a single service account (e.g., `marmot@my-gcp-project.iam.gserviceaccount.com`) permission to list buckets, then run Marmot _as_ that account.

<Steps>
<Step title="Create the service account and grant it access">

```hcl
# Create the service account
resource "google_service_account" "marmot" {
  project    = "my-gcp-project"
  account_id = "marmot"
}

# Grant it permission to discover buckets
resource "google_project_iam_member" "marmot_bucket_viewer" {
  project = "my-gcp-project"
  role    = "roles/storage.bucketViewer"
  member  = "serviceAccount:${google_service_account.marmot.email}"
}
```

See [Required Permissions](#required-permissions) for the exact role and its alternatives.

</Step>
<Step title="Authenticate as the service account">

Pick the tab for where Marmot runs.

<Tabs items={[
{ label: "GKE", value: "gke", icon: "mdi:kubernetes" },
{ label: "Cloud Run", value: "run", icon: "mdi:google-cloud" },
{ label: "Other platform", value: "external", icon: "mdi:server" }
]}>
<TabPanel>

Bind Marmot's Kubernetes service account to the Google service account with [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity). Credentials resolve automatically, so the plugin config is just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
<TabPanel>

Attach the service account to the Cloud Run service. The metadata server provides credentials, so they resolve automatically and the plugin config is just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
<TabPanel>

Running outside Google Cloud, on another cloud, on premises, or in CI? Use [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) so the platform's own identity (an AWS role, an Azure managed identity, or any OIDC or SAML token) can impersonate the service account without an exported key. Create a Google identity pool/provider that trusts your identity provider, grant that external identity the `roles/iam.workloadIdentityUser` role on the service account, then generate a credential configuration file and point `GOOGLE_APPLICATION_CREDENTIALS` at it. If you're running on AWS or Azure, see Google's guides for [VMs](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-other-clouds#gcloud_1) or [EKS and AKS](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes). The plugin config is then just:

```yaml
project_id: "my-gcp-project"
include_metadata: true
```

</TabPanel>
</Tabs>

</Step>
</Steps>

### Precedence

When multiple credential options are set, the plugin resolves them in this order:

1. `disable_auth`: skips authentication entirely; for local emulators only.
2. `credentials_json`: inline service account key.
3. `credentials_file`: service account key file.
4. **Application Default Credentials**: used when none of the above are set (recommended).

`impersonate_service_account` is independent: whichever identity is resolved above is used only to issue temporary tokens for the target account (the base identity must have `roles/iam.serviceAccountTokenCreator` on the target service account); no key is ever exported.

<TipBox variant="warning" title="Avoid service account keys">
Exported keys are persistent secrets that Google <a href="https://cloud.google.com/iam/docs/best-practices-for-managing-service-account-keys">recommends against</a>.
</TipBox>

## Required Permissions

The service account needs the following IAM roles:

- **Storage Bucket Viewer** (`roles/storage.bucketViewer`) - For discovering and listing buckets

Or use a custom role with these permissions:
- `storage.buckets.list`
- `storage.buckets.get`
- `storage.objects.list` (if using object count)



## Example Configuration

```yaml

project_id: "my-gcp-project"
# Authentication uses Application Default Credentials by default.
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
| credentials_file | string | false | Path to a service account JSON key file. |
| credentials_json | string | false | Service account JSON key content. |
| disable_auth | bool | false | Disable authentication (for local emulators) |
| endpoint | string | false | Custom endpoint URL (for fake-gcs-server or other emulators) |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| impersonate_service_account | string | false | Email of a service account to impersonate (requires `roles/iam.serviceAccountTokenCreator` on the target service account). |
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