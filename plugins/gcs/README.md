The Google Cloud Storage plugin discovers buckets from GCP projects. It captures bucket metadata including location, storage class, encryption settings, and lifecycle rules.

## Connection Examples

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

- **Storage Bucket Viewer** (`roles/storage.bucketViewer`) - For discovering and listing buckets

Or use a custom role with these permissions:
- `storage.buckets.list`
- `storage.buckets.get`
- `storage.objects.list` (if using object count)

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of a service account key. Set `workload_identity_provider` to a Workload Identity Federation provider that trusts your Marmot instance as an OIDC issuer, and grant the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, the role above directly (`principal://iam.googleapis.com/<pool>/subject/pipeline:<name>`), or grant it `roles/iam.workloadIdentityUser` on a service account named in `service_account`. No key exists anywhere; Marmot mints a short-lived token for each run.
