The BigQuery plugin discovers datasets, tables, views, and external tables from Google BigQuery projects. It captures schemas, statistics, and lineage relationships.

## Required Permissions

Assign `roles/bigquery.metadataViewer` to your service account, or these individual permissions:

- `bigquery.datasets.get`
- `bigquery.tables.get`
- `bigquery.tables.list`

## Keyless authentication

On Marmot Cloud or Marmot Enterprise the pipeline can present its own identity instead of a service account key. Set `workload_identity_provider` to a Workload Identity Federation provider that trusts your Marmot instance as an OIDC issuer, and grant the pipeline's subject, `pipeline:<name>` as reported by the pipeline API, the role above directly (`principal://iam.googleapis.com/<pool>/subject/pipeline:<name>`), or grant it `roles/iam.workloadIdentityUser` on a service account named in `service_account`. No key exists anywhere; Marmot mints a short-lived token for each run.

## Querying

The plugin can run read-only data queries against this source. Every statement is first validated with a BigQuery dry run, which is free and has no side effects: anything that is not a plain `SELECT`, including scripts, DDL, DML and `EXPORT DATA` statements, is refused before it ever reaches a real job. The same dry run maps the tables and views the statement reads to the assets discovery catalogued, so grants written against those assets apply to the query too; a statement that calls `EXTERNAL_QUERY` is treated as reading more than the plan can name, since the external database's own tables never appear in it.

BigQuery has no read-only session or transaction to run a statement under, so the real boundary is the service account's IAM: grant it `roles/bigquery.dataViewer` on the datasets it may query and `roles/bigquery.jobUser` on the billing project, and nothing more. `max_bytes_billed` additionally caps the bytes a single query may bill for, 10GiB by default; BigQuery fails the query rather than exceed it. Set it to `0` to fall back to the project's own default limit instead.
