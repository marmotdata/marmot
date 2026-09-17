The BigQuery plugin discovers datasets, tables, views, and external tables from Google BigQuery projects. It captures schemas, statistics, and lineage relationships.

## Required Permissions

Assign `roles/bigquery.metadataViewer` to your service account, or these individual permissions:

- `bigquery.datasets.get`
- `bigquery.tables.get`
- `bigquery.tables.list`

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of a service account key. Set `workload_identity_provider` to a Workload Identity Federation provider that trusts your Marmot instance as an OIDC issuer, and grant the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, the role above directly (`principal://iam.googleapis.com/<pool>/subject/pipeline:<name>`), or grant it `roles/iam.workloadIdentityUser` on a service account named in `service_account`. No key exists anywhere; Marmot mints a short-lived token for each run.
