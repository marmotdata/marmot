---
sidebar_position: 1
title: Connect Google Cloud
sidebar_label: Google Cloud
description: Your first pipeline on Marmot Cloud, cataloging BigQuery with no service account key.
---

import { Steps, Step } from '@site/src/components/Steps';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Connect Google Cloud

Catalog BigQuery every 6 hours with a pipeline that authenticates to Google Cloud as itself, so there's no service account key to create, store or rotate.

## Before you start

- You hold `ingestion.admin` across the whole catalog in your instance.
- You have a Google Cloud project where you can create Workload Identity pools and IAM bindings. This page calls it `analytics-prod`.

## How it works

Before each run, your instance gives the pipeline a 5-minute token with the subject `pipeline:<pipeline-name>`. Workload Identity Federation exchanges it for Google credentials, so you grant each pipeline exactly the access it needs. [Keyless pipelines](../pipelines.md#how-keyless-pipelines-work) explains the model.

## Values you'll need

The steps use these values. Replace `<instance>` and `<project-number>` with your own:

- **Issuer URL**: `https://<instance>.marmotdata.cloud`, with no trailing slash. Always the `marmotdata.cloud` address, even with a custom domain
- **Pool**: `projects/<project-number>/locations/global/workloadIdentityPools/marmot`, written `$POOL` below. It uses the project number, not the project ID
- **Provider**: `$POOL/providers/marmot`
- **Pipeline principal**: `principal://iam.googleapis.com/$POOL/subject/pipeline:analytics`

The commands read the pool from a shell variable:

```bash
POOL=projects/<project-number>/locations/global/workloadIdentityPools/marmot
```

## Your first pipeline

<Steps>
<Step title="Trust your instance">

Create one Workload Identity pool with an OIDC provider for your instance. Keep the default audience.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. Open **IAM & Admin → Workload Identity Federation** and create a pool named `marmot`.
2. Under **Select a provider**, choose **OpenID Connect (OIDC)**, name it `marmot` and enter the issuer URL.
3. Under **Configure provider attributes**, map `google.subject` to `assertion.sub` and select **Save**.

  </TabItem>
  <TabItem value="cli" label="gcloud">

```bash
gcloud iam workload-identity-pools create marmot \
  --project=analytics-prod --location=global

gcloud iam workload-identity-pools providers create-oidc marmot \
  --project=analytics-prod --location=global \
  --workload-identity-pool=marmot \
  --issuer-uri="https://<instance>.marmotdata.cloud" \
  --attribute-mapping="google.subject=assertion.sub"
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
provider "marmot" {
  host = "https://<instance>.marmotdata.cloud"
}

provider "google" {
  project = "analytics-prod"
}

resource "google_iam_workload_identity_pool" "marmot" {
  workload_identity_pool_id = "marmot"
}

locals {
  pool      = google_iam_workload_identity_pool.marmot
  principal = "principal://iam.googleapis.com/${local.pool.name}/subject"
}

resource "google_iam_workload_identity_pool_provider" "marmot" {
  workload_identity_pool_id          = local.pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "marmot"
  attribute_mapping                  = { "google.subject" = "assertion.sub" }

  oidc {
    issuer_uri = "https://<instance>.marmotdata.cloud"
  }
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Grant the pipeline access">

Grant the pipeline principal `roles/bigquery.metadataViewer` on the project. It reads dataset and table structure but no rows.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. Open **IAM & Admin → IAM** for `analytics-prod` and select **Grant access**.
2. Enter the pipeline principal in **New principals**.
3. Choose **BigQuery Metadata Viewer** and select **Save**.

  </TabItem>
  <TabItem value="cli" label="gcloud">

```bash
gcloud projects add-iam-policy-binding analytics-prod \
  --role=roles/bigquery.metadataViewer \
  --member="principal://iam.googleapis.com/$POOL/subject/pipeline:analytics"
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

The subject comes from the pipeline you declare in the next step, so a rename updates the grant too:

```hcl
resource "google_project_iam_member" "marmot_reads_bigquery" {
  project = "analytics-prod"
  role    = "roles/bigquery.metadataViewer"
  member  = "${local.principal}/${marmot_pipeline.analytics.subject}"
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Create the pipeline">

Point the BigQuery plugin at your project and set the provider. The provider is what tells the plugin to sign in as the pipeline rather than look for a key.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Plugins** and select **Google BigQuery** under **Add a source**.
2. Enter `analytics` as the **Pipeline name** and select **Continue**.
3. Set **Project ID** to `analytics-prod` and **Workload Identity Provider** to the provider, leave the key fields empty and select **Check and continue**.
4. Turn on **Run on a schedule**, set it to `0 */6 * * *` and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-bigquery-federated-light.png"
  darkSrc="/img/cloud-bigquery-federated-dark.png"
  alt="The BigQuery pipeline's connection settings with Project ID analytics-prod and the Workload Identity Provider filled in and no key"
/>

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_pipeline" "analytics" {
  name            = "analytics"
  plugin_id       = "bigquery"
  cron_expression = "0 */6 * * *"

  config = jsonencode({
    project_id = "analytics-prod"
    workload_identity_provider = (
      google_iam_workload_identity_pool_provider.marmot.name
    )
  })
}
```

  </TabItem>
</Tabs>

</Step>
</Steps>

## Check that it worked

The pipeline runs at its next scheduled time. To run it now, open it under **Plugins** and select **Run now**. When the run succeeds, your datasets and tables appear in **Discover**.

From the CLI, `marmot runs list --pipelines analytics` lists its runs.

## Other Google Cloud sources

Every other Google Cloud plugin works the same way: set the provider in its config and grant the pipeline principal a read-only role. For Cloud Storage that's `roles/storage.bucketViewer` on the project. Each plugin's page in the [plugin directory](https://plugins.marmotdata.io) lists its configuration.

## When a source needs a password

A Cloud SQL or self-managed Postgres can't federate, so its pipeline needs a real password. Keep it in Secret Manager and let Marmot read it at run time through a [secret store](../secret-stores.md). The store signs in through the same pool, as the subject `secretStore:<store-name>`.

<Steps>
<Step title="Grant the store the secret">

Grant `roles/secretmanager.secretAccessor` on the one secret, not the project. The store will be called `gcp-prod`.

<Tabs groupId="cloud-method" queryString="cloud-method">
  <TabItem value="console" label="Console" default>

1. Open **Secret Manager** and select the checkbox next to `orders-db-password`.
2. In the info panel, select **Add Principal** and enter `principal://iam.googleapis.com/$POOL/subject/secretStore:gcp-prod` with your pool in place of `$POOL`.
3. Choose **Secret Manager Secret Accessor** and save.

  </TabItem>
  <TabItem value="cli" label="gcloud">

```bash
gcloud secrets add-iam-policy-binding orders-db-password \
  --project=analytics-prod \
  --role=roles/secretmanager.secretAccessor \
  --member="principal://iam.googleapis.com/$POOL/subject/secretStore:gcp-prod"
```

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "google_secret_manager_secret" "orders_db_password" {
  secret_id = "orders-db-password"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "marmot_reads_orders" {
  secret_id = google_secret_manager_secret.orders_db_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "${local.principal}/${marmot_secret_store_google.prod.subject}"
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Create the store and register the secret">

Create a store named `gcp-prod` that signs in through the same provider, then register the secret in it.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Settings → Secret stores** and select **New secret store**.
2. Enter `gcp-prod` as the **Name** and choose **Google Secret Manager** under **Where do the secrets live?**.
3. Set **Workload Identity Provider** to the provider and select **Create secret store**.
4. On the store's page, select **Register secret**, enter `analytics-prod` as the **Project** and `orders-db-password` as the **Secret ID**, then select **Register secret**.

<ThemedImg
  lightSrc="/img/cloud-new-secret-store-light.png"
  darkSrc="/img/cloud-new-secret-store-dark.png"
  alt="The New secret store form named gcp-prod, with Google Secret Manager chosen, the Workload Identity Provider filled in and the subject secretStore:gcp-prod"
/>

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_secret_store_google" "prod" {
  name                       = "gcp-prod"
  workload_identity_provider = (
    google_iam_workload_identity_pool_provider.marmot.name
  )
}

resource "marmot_secret_store_google_secret" "orders_db_password" {
  store     = marmot_secret_store_google.prod.id
  project   = google_secret_manager_secret.orders_db_password.project
  secret_id = google_secret_manager_secret.orders_db_password.secret_id
}
```

  </TabItem>
</Tabs>

</Step>
<Step title="Use the secret in a pipeline">

Create the PostgreSQL pipeline with its password read from the store. No version is pinned, so rotating the password is adding a version in Secret Manager.

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Plugins** and select **PostgreSQL** under **Add a source**.
2. Enter `orders` as the **Pipeline name** and select **Continue**.
3. Fill in **Host** and **User**. For **Password**, choose **From secret store** and pick the secret.
4. Select **Check and continue**, set a schedule and select **Create pipeline**.

<ThemedImg
  lightSrc="/img/cloud-postgres-secret-light.png"
  darkSrc="/img/cloud-postgres-secret-dark.png"
  alt="The PostgreSQL pipeline's connection settings with the password read from the orders-db-password secret in gcp-prod"
/>

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_pipeline" "orders" {
  name            = "orders"
  plugin_id       = "postgresql"
  cron_expression = "0 * * * *"

  config = jsonencode({
    host = "orders-db.example.com"
    user = "marmot_reader"
  })

  secrets = {
    password = marmot_secret_store_google_secret.orders_db_password.id
  }
}
```

  </TabItem>
</Tabs>

</Step>
</Steps>

## Troubleshooting

| What you see | What to do |
| --- | --- |
| `workload_identity_provider must name a provider as projects/<number>/...` | Use the project number in the provider, not the project ID. |
| The token exchange is rejected | Check the provider's issuer is exactly the issuer URL above. |
| The exchange succeeds but the API call is denied | The binding names a different subject, usually after a rename. Grant `pipeline:<pipeline-name>` again. |
| Everything is denied right after you grant access | IAM changes take a few minutes to propagate. Wait and run the pipeline again. |

## Next steps

- [Grant and check access](../grant-access.md): decide who can see the new datasets.
- [Secret stores](../secret-stores.md): more about stores, Vault and rotation.
- [Connect AWS](aws.md) or [Connect Azure](azure.md) if you catalog more than one cloud.
