The Cloud Run plugin discovers services and jobs from a Google Cloud project using the Cloud Run Admin API v2. It records the deployed image, scaling, networking and volume configuration of each workload, the execution count of each job, and the recent executions of each job as run history.

Every region is scanned in one call unless `locations` is set. Regions Cloud Run reports as unreachable are logged and skipped rather than failing the run.

## Connection Examples

<Collapsible title="Service Account File" icon="logos:google-cloud">

```yaml
project_id: "acme-prod"
credentials_file: "/path/to/service-account.json"
include_jobs: true
include_executions: true
tags:
  - "cloudrun"
```

</Collapsible>

<Collapsible title="Selected Regions" icon="mdi:map-marker">

```yaml
project_id: "acme-prod"
credentials_json: "${CLOUDRUN_CREDENTIALS_JSON}"
locations:
  - "europe-west1"
  - "us-central1"
max_executions_per_job: 25
filter:
  include:
    - "^europe-west1/.*"
```

</Collapsible>

## Lineage

Cloud Run only reveals a data dependency through a volume mount. A Cloud Storage volume becomes a `FEEDS` edge from the bucket to the service or job that mounts it, matching the identity the `gcs` plugin gives that bucket.

Cloud SQL volumes are recorded in the `cloud_sql_instances` metadata field but produce no edge: the API does not say which engine an instance runs, so there is no provider to address it by.

## Required Permissions

The service account needs the following IAM role:

- **Cloud Run Viewer** (`roles/run.viewer`) - For listing services, jobs and executions

Or use a custom role with these permissions:
- `run.services.list`
- `run.jobs.list`
- `run.executions.list`

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of a service account key. Set `workload_identity_provider` to a Workload Identity Federation provider that trusts your Marmot instance as an OIDC issuer, and grant the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, the role above directly (`principal://iam.googleapis.com/<pool>/subject/pipeline:<name>`), or grant it `roles/iam.workloadIdentityUser` on a service account named in `service_account`. No key exists anywhere; Marmot mints a short-lived token for each run.
