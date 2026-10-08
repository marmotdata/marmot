---
sidebar_position: 1
title: Google Cloud
description: Your first pipeline on Marmot Cloud, cataloging BigQuery with no service account key.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { Steps, Step } from '@site/src/components/Steps';

# Google Cloud

This guide takes you from an empty instance to a pipeline that catalogs BigQuery every six hours. The pipeline authenticates to Google Cloud as itself, so there is no service account key to create, store or rotate.

You need a Marmot Cloud instance, written as `https://acme.marmotdata.cloud` throughout, a project where you can create Workload Identity pools and IAM bindings, and Terraform with the `marmot` and `google` providers. If you have not signed in from the CLI yet, do [getting started](../getting-started.md) first.

## How it works

Your instance is an OIDC issuer. Workload Identity Federation trusts tokens from an issuer you register and exchanges them for Google credentials. Every pipeline presents its own name as the token's subject, so you grant each pipeline exactly the access it needs.

## Your first pipeline

<Steps>
<Step title="Configure the providers">

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    marmot = { source = "marmotdata/marmot" }
    google = { source = "hashicorp/google" }
  }
}

provider "marmot" {
  host = "https://acme.marmotdata.cloud"
}

provider "google" {
  project = "acme-prod"
}
```

</Step>
<Step title="Trust your instance">

One pool per Marmot instance. The provider trusts your instance's URL and maps the token's subject to a Google principal, which is what lets you grant to one pipeline at a time.

```hcl
resource "google_iam_workload_identity_pool" "marmot" {
  workload_identity_pool_id = "marmot"
  display_name              = "Marmot Cloud"
}

resource "google_iam_workload_identity_pool_provider" "marmot" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.marmot.workload_identity_pool_id
  workload_identity_pool_provider_id = "marmot"

  attribute_mapping = {
    "google.subject" = "assertion.sub"
  }

  oidc {
    issuer_uri = "https://acme.marmotdata.cloud"
  }
}
```

</Step>
<Step title="Declare the pipeline">

Point the BigQuery plugin at your project and name the provider you just created. That is what tells the plugin to authenticate as the pipeline rather than look for a key.

```hcl
resource "marmot_pipeline" "analytics" {
  name      = "analytics"
  plugin_id = "bigquery"

  config = jsonencode({
    project_id                 = "acme-analytics-prod"
    workload_identity_provider = google_iam_workload_identity_pool_provider.marmot.name
  })

  cron_expression = "0 */6 * * *"
}
```

</Step>
<Step title="Grant it access">

The pipeline's subject comes from the resource, so a rename updates the grant too. Metadata viewer is enough: Marmot reads dataset and table structure, never rows.

```hcl
resource "google_project_iam_member" "marmot_reads_bigquery_metadata" {
  project = "acme-analytics-prod"
  role    = "roles/bigquery.metadataViewer"
  member  = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.marmot.name}/subject/${marmot_pipeline.analytics.subject}"
}
```

</Step>
<Step title="Apply and run">

```bash
terraform apply
marmot runs list
```

The pipeline runs on its next tick, or now from the Pipelines page in your instance. When the run completes, your datasets and tables are in the catalog.

</Step>
</Steps>

Every other Google Cloud plugin works the same way: name the provider in the config and grant the pipeline's subject the read-only role. For Cloud Storage that is `roles/storage.objectViewer` on the buckets to catalog. Each plugin's page on the [plugin registry](https://plugins.marmotdata.io) names its role.

## When a source needs a password

A Cloud SQL or self-managed Postgres cannot federate, so its pipeline needs a real password. Keep it in Secret Manager and let Marmot read it at run time through a secret store. The store authenticates through the same pool as your pipelines.

```hcl
resource "marmot_secret_store_google" "prod" {
  name                       = "gcp-prod"
  workload_identity_provider = google_iam_workload_identity_pool_provider.marmot.name
}

resource "google_secret_manager_secret" "orders_db_password" {
  secret_id = "orders-db-password"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "marmot_reads_orders_password" {
  secret_id = google_secret_manager_secret.orders_db_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.marmot.name}/subject/${marmot_secret_store_google.prod.subject}"
}

resource "marmot_secret_store_google_secret" "orders_db_password" {
  store     = marmot_secret_store_google.prod.id
  project   = google_secret_manager_secret.orders_db_password.project
  secret_id = google_secret_manager_secret.orders_db_password.secret_id
}

resource "marmot_pipeline" "orders" {
  name      = "orders"
  plugin_id = "postgresql"

  config = jsonencode({
    host     = "orders-db.acme.internal"
    database = "orders"
    user     = "marmot"
  })

  secrets = {
    password = marmot_secret_store_google_secret.orders_db_password.id
  }

  cron_expression = "0 * * * *"
}
```

Grant on the individual secret, not the project. No version is pinned, so rotating the password is adding a version in Secret Manager. [Secret stores](../secret-stores.md) covers the model.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Token exchange rejected | The issuer URL does not exactly match your instance URL. A trailing slash is enough. |
| Exchange succeeds, API call denied | The IAM binding names a different subject, usually because the pipeline was renamed. |
| Everything denied right after an apply | IAM propagation. Wait a minute. |

<CalloutCard
  title="Next: decide who sees what"
  description="Grant roles per asset, data product, glossary term and secret store."
  docId="Cloud/access-control"
  buttonText="Access control"
  icon="mdi:shield-key"
/>
