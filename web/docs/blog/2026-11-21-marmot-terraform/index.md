---
slug: marmot-terraform
title: "Marmot now supports Terraform"
authors:
  - name: Bruno Schaatsbergen
    url: https://github.com/bschaatsbergen
image: /img/marmot-terraform-banner.png
description: "The Marmot Terraform provider manages pipelines, secret stores, service accounts and access grants as code, next to the infrastructure they describe. A step-by-step setup on Google Cloud, AWS and Azure, and how the provider is built."
tags: [terraform, infrastructure-as-code, opentofu, pulumi, marmot-cloud]
keywords:
  - terraform data catalog
  - data catalog as code
  - marmot terraform provider
  - catalog as code
  - opentofu data catalog
---

import { CalloutCard } from '@site/src/components/DocCard';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<div style={{textAlign: 'center', marginBottom: '2rem'}}>
  <img src="/img/marmot-terraform-banner.png" alt="The Marmot mascot next to the Terraform logo, over the Plugins page" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

Marmot finds your tables, topics and services by itself. What it can't decide for you is which sources to watch, how often, with which credentials, and who gets to see the result. Those are decisions, and decisions clicked into a UI drift, get forgotten and never go through review.

Marmot ships a Terraform provider so those decisions live in a repository instead. Pipelines, secret stores, service accounts and access grants are resources, declared next to the BigQuery dataset, the IAM role or the Key Vault they refer to, and applied in the same run. It is on the [Terraform Registry](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs) and the [OpenTofu Registry](https://search.opentofu.org/provider/marmotdata/marmot/latest), and it works from Pulumi too.

<!-- truncate -->

## What belongs in Terraform

The rule of thumb from the provider's [getting started guide](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/guides/getting-started): a pipeline discovers, Terraform decides.

| Terraform declares | A pipeline discovers |
| --- | --- |
| Pipelines: which plugin, which source, what schedule | Assets: tables, topics, buckets, services |
| Secret stores and the secrets pipelines read | Schemas and columns |
| Service accounts for agents, CI and ingestion | Lineage a plugin can see for itself |
| Access grants: who reaches what | Technical descriptions |
| Data products, glossary terms, teams and ownership | |

You can declare assets and lineage by hand with `marmot_asset` and `marmot_lineage`, and that is right for a handful of things you own. For a warehouse with ten thousand tables, point a pipeline at it.

The walkthrough below sets up the first three rows on Marmot Cloud: a keyless pipeline, a password read from your own vault, and an access grant. Pick your cloud in each step. The examples are the ones from the [Google Cloud](/docs/Cloud/Guides/google), [AWS](/docs/Cloud/Guides/aws) and [Azure](/docs/Cloud/Guides/azure) guides, which also show every step in the console and the CLI.

---

## Step 1: Point the provider at your instance

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    marmot = {
      source = "marmotdata/marmot"
    }
  }
}

provider "marmot" {
  host = "https://<instance>.marmotdata.cloud"
}
```

The block has no credential in it, and usually that's right. The provider takes an API key from `api_key` or `MARMOT_API_KEY`, a bearer token from `token` or `MARMOT_TOKEN`, and with neither it uses the session `marmot login` wrote. So on your machine you sign in once, and in CI you set one environment variable.

<Tabs groupId="auth" queryString="auth">
  <TabItem value="laptop" label="Your machine" default>

```bash
marmot login https://<instance>.marmotdata.cloud
terraform plan
```

The CLI keeps a short-lived token, so there is no key on disk.

  </TabItem>
  <TabItem value="ci" label="CI">

Give CI a [service account of its own](/docs/Cloud/service-accounts#run-terraform-in-ci) and put its key in the environment. The provider block is then empty:

```hcl
provider "marmot" {}
```

```yaml
- name: Terraform apply
  env:
    MARMOT_HOST: https://<instance>.marmotdata.cloud
    MARMOT_API_KEY: ${{ secrets.MARMOT_API_KEY }}
  run: terraform apply -auto-approve
```

  </TabItem>
  <TabItem value="vault" label="Secret manager">

If the key already lives in a secret manager, read it through an ephemeral resource. Ephemeral resources exist since Terraform 1.10, and their values never reach the plan or the state:

```hcl
ephemeral "google_secret_manager_secret_version" "marmot_api_key" {
  secret  = "marmot-api-key"
  version = "latest"
}

provider "marmot" {
  host    = "https://<instance>.marmotdata.cloud"
  api_key = ephemeral.google_secret_manager_secret_version.marmot_api_key.secret_data
}
```

AWS Secrets Manager and HashiCorp Vault have ephemeral resources that work the same way.

  </TabItem>
</Tabs>

---

## Step 2: Declare a keyless pipeline

On Marmot Cloud a pipeline signs in to your cloud as itself, with no key, as [keyless auth](/docs/Cloud/pipelines#how-keyless-pipelines-work) explains. In Terraform that is the trust on your cloud's side and the pipeline on Marmot's, in one configuration. The `marmot_pipeline` resource exposes `issuer` and `subject`, so the grant can reference the pipeline and a rename updates both sides in one apply.

<Tabs groupId="cloud" queryString="cloud">
  <TabItem value="google" label="Google Cloud" default>

A Workload Identity pool that trusts your instance, a BigQuery Metadata Viewer grant to the pipeline's principal, and the pipeline:

```hcl
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

resource "google_project_iam_member" "marmot_reads_bigquery" {
  project = "analytics-prod"
  role    = "roles/bigquery.metadataViewer"
  member  = "${local.principal}/${marmot_pipeline.analytics.subject}"
}

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
  <TabItem value="aws" label="AWS">

An IAM OIDC provider that trusts your instance, a role only the `lake` pipeline can assume with read-only Glue access, and the pipeline. The name lives in a local because the trust policy needs it before the pipeline exists:

```hcl
provider "aws" {
  region = "eu-west-1"
}

locals {
  marmot_host        = "<instance>.marmotdata.cloud"
  lake_pipeline_name = "lake"
}

resource "aws_iam_openid_connect_provider" "marmot" {
  url            = "https://<instance>.marmotdata.cloud"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_policy_document" "marmot_lake_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.marmot.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:sub"
      values   = ["pipeline:${local.lake_pipeline_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "marmot_lake" {
  name               = "marmot-lake"
  assume_role_policy = data.aws_iam_policy_document.marmot_lake_trust.json
}

resource "aws_iam_role_policy" "marmot_lake_read" {
  name = "glue-read"
  role = aws_iam_role.marmot_lake.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "glue:GetDatabases",
        "glue:GetTables",
        "glue:GetJobs",
        "glue:GetCrawlers",
      ]
      Resource = "*"
    }]
  })
}

resource "marmot_pipeline" "lake" {
  name            = local.lake_pipeline_name
  plugin_id       = "glue"
  cron_expression = "0 */6 * * *"

  config = jsonencode({
    credentials = {
      region   = "eu-west-1"
      role_arn = aws_iam_role.marmot_lake.arn
    }
  })
}
```

  </TabItem>
  <TabItem value="azure" label="Azure">

An app registration with no secret, a federated credential that names the pipeline, Storage Blob Data Reader on the storage account, and the pipeline. The credential references the pipeline's `issuer` and `subject`:

```hcl
provider "azurerm" {
  features {}
}

data "azurerm_client_config" "current" {}

resource "azuread_application" "marmot_blob" {
  display_name = "marmot-blob-pipeline"
}

resource "azuread_service_principal" "marmot_blob" {
  client_id = azuread_application.marmot_blob.client_id
}

resource "azuread_application_federated_identity_credential" "marmot_blob" {
  application_id = azuread_application.marmot_blob.id
  display_name   = "marmot-blobs"
  issuer         = marmot_pipeline.blobs.issuer
  subject        = marmot_pipeline.blobs.subject
  audiences      = ["api://AzureADTokenExchange"]
}

data "azurerm_storage_account" "orders_archive" {
  name                = "ordersarchive"
  resource_group_name = "data-prod"
}

resource "azurerm_role_assignment" "marmot_reads_blobs" {
  scope                = data.azurerm_storage_account.orders_archive.id
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = azuread_service_principal.marmot_blob.object_id
}

resource "marmot_pipeline" "blobs" {
  name            = "blobs"
  plugin_id       = "azureblob"
  cron_expression = "0 */6 * * *"

  config = jsonencode({
    account_name = "ordersarchive"
    tenant_id    = data.azurerm_client_config.current.tenant_id
    client_id    = azuread_application.marmot_blob.client_id
  })
}
```

  </TabItem>
</Tabs>

The config is whatever the plugin accepts, written in HCL and passed through `jsonencode`. The server validates it against the plugin at apply time, so an invalid config fails the apply instead of the first run. The [plugin directory](https://plugins.marmotdata.io) lists each plugin's settings.

---

## Step 3: Read a password from your vault

A Postgres on Cloud SQL, RDS or Azure Database can't federate, so its pipeline needs a password. Keep it in your secret manager and declare a [secret store](/docs/Cloud/secret-stores): the store is a connection to one secret manager with an identity of its own, `secretStore:<name>`, and a secret is the address of one value in it. Marmot stores the address, never the value, and reads it right before each run.

<Tabs groupId="cloud" queryString="cloud">
  <TabItem value="google" label="Google Cloud" default>

The store signs in through the pool from step 2. Grant it Secret Manager Secret Accessor on the one secret, not the project. The subject comes from the store resource, so you don't copy it by hand:

```hcl
resource "marmot_secret_store_google" "prod" {
  name                       = "gcp-prod"
  workload_identity_provider = (
    google_iam_workload_identity_pool_provider.marmot.name
  )
}

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

resource "marmot_secret_store_google_secret" "orders_db_password" {
  store     = marmot_secret_store_google.prod.id
  project   = google_secret_manager_secret.orders_db_password.project
  secret_id = google_secret_manager_secret.orders_db_password.secret_id
}

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
  <TabItem value="aws" label="AWS">

The store assumes a role of its own through the provider from step 2, with a trust policy pinned to `secretStore:aws-prod` and `secretsmanager:GetSecretValue` on the one secret:

```hcl
locals {
  secret_store_name = "aws-prod"
}

data "aws_iam_policy_document" "marmot_store_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.marmot.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:sub"
      values   = ["secretStore:${local.secret_store_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.marmot_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "marmot_store" {
  name               = "marmot-secret-store"
  assume_role_policy = data.aws_iam_policy_document.marmot_store_trust.json
}

resource "aws_secretsmanager_secret" "orders_db_password" {
  name = "prod/orders/db-password"
}

resource "aws_iam_role_policy" "marmot_reads_orders_password" {
  name = "read-orders-password"
  role = aws_iam_role.marmot_store.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "secretsmanager:GetSecretValue"
      Resource = aws_secretsmanager_secret.orders_db_password.arn
    }]
  })
}

resource "marmot_secret_store_aws" "prod" {
  name     = local.secret_store_name
  role_arn = aws_iam_role.marmot_store.arn
}

resource "marmot_secret_store_aws_secret" "orders_db_password" {
  store     = marmot_secret_store_aws.prod.id
  secret_id = aws_secretsmanager_secret.orders_db_password.arn
}

resource "marmot_pipeline" "orders" {
  name            = "orders"
  plugin_id       = "postgresql"
  cron_expression = "0 * * * *"

  config = jsonencode({
    host = "orders-db.example.com"
    user = "marmot_reader"
  })

  secrets = {
    password = marmot_secret_store_aws_secret.orders_db_password.id
  }
}
```

  </TabItem>
  <TabItem value="azure" label="Azure">

The store gets an app registration of its own with a federated credential for `secretStore:azure-prod`, and Key Vault Secrets User on the vault. The credential references the store's `issuer`, `subject` and `audience`:

```hcl
resource "azuread_application" "marmot_secrets" {
  display_name = "marmot-secret-store"
}

resource "azuread_service_principal" "marmot_secrets" {
  client_id = azuread_application.marmot_secrets.client_id
}

resource "azuread_application_federated_identity_credential" "marmot_secrets" {
  application_id = azuread_application.marmot_secrets.id
  display_name   = "marmot-azure-prod"
  issuer         = marmot_secret_store_azure.prod.issuer
  subject        = marmot_secret_store_azure.prod.subject
  audiences      = [marmot_secret_store_azure.prod.audience]
}

data "azurerm_key_vault" "orders" {
  name                = "orders-vault"
  resource_group_name = "data-prod"
}

resource "azurerm_role_assignment" "marmot_reads_secrets" {
  scope                = data.azurerm_key_vault.orders.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = azuread_service_principal.marmot_secrets.object_id
}

resource "marmot_secret_store_azure" "prod" {
  name      = "azure-prod"
  tenant_id = data.azurerm_client_config.current.tenant_id
  client_id = azuread_application.marmot_secrets.client_id
}

resource "marmot_secret_store_azure_secret" "orders_db_password" {
  store     = marmot_secret_store_azure.prod.id
  vault_uri = data.azurerm_key_vault.orders.vault_uri
  name      = "orders-db-password"
}

resource "marmot_pipeline" "orders" {
  name            = "orders"
  plugin_id       = "postgresql"
  cron_expression = "0 * * * *"

  config = jsonencode({
    host = "orders-db.example.com"
    user = "marmot_reader"
  })

  secrets = {
    password = marmot_secret_store_azure_secret.orders_db_password.id
  }
}
```

  </TabItem>
</Tabs>

The `secrets` map names a config key and the secret that fills it, and that key is left out of `config`. The password is not in the configuration, the plan or the state. Rotate it in your secret manager and the next run uses the new value. Terraform doesn't see a change, because it only has the reference.

---

## Step 4: Grant access

Agents, CI runners and ingestion jobs all talk to the catalog, and none of them should see more than they need. Each one gets a [service account](/docs/Cloud/service-accounts), and a new service account can sign in and do nothing else. You add grants from there.

Grants come in three resources per scope, and the difference is how much of the policy Terraform owns. `_iam_member` adds one member to one role and leaves everything else alone. `_iam_binding` owns one role and removes members of it that aren't in the configuration. `_iam_policy` owns the whole policy. Start with `_iam_member`, as the provider's [access control guide](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/guides/access-control) recommends. You can grant on the organization, a data product, an asset, a glossary term or a secret store, and grants inherit downwards: a grant on a data product covers the assets in it, now and later.

```hcl
resource "marmot_service_account" "copilot" {
  name        = "catalog-copilot"
  description = "Answers questions in #data-help. Owned by the platform team."
}

resource "marmot_service_account" "ingest" {
  name        = "orders-ingest"
  description = "Attaches secrets to the orders pipelines. Owned by the platform team."
}

resource "marmot_data_product" "revenue" {
  name        = "Revenue"
  description = "Everything the finance team reconciles against."
}

resource "marmot_organization_iam_member" "everyone_reads" {
  role   = "catalog.viewer"
  member = "allAuthenticated"
}

resource "marmot_data_product_iam_member" "copilot_reads_revenue" {
  data_product_id = marmot_data_product.revenue.id
  role            = "dataProduct.viewer"
  member          = "serviceAccount:${marmot_service_account.copilot.id}"
}

resource "marmot_secret_store_iam_member" "ingest_uses_prod" {
  secret_store_id = marmot_secret_store_google.prod.id
  role            = "secretStore.user"
  member          = "serviceAccount:${marmot_service_account.ingest.id}"
}
```

Everyone signed in can read the catalog. The copilot can also read one data product. The ingest account has no role at the root and can reach one secret store, where `secretStore.user` lets it attach secrets to pipelines without ever reading a value. Removing that one resource takes the access away. The store is the Google one from step 3; on AWS or Azure, reference `marmot_secret_store_aws.prod` or `marmot_secret_store_azure.prod` instead.

There are no deny rules, so keep the catalog-wide grant narrow and [lower the base role](/docs/Cloud/grant-access#set-the-base-role) if the catalog holds something not everyone should read. Policy writes carry an etag: if someone changes a policy in the UI between your plan and your apply, the apply is rejected instead of overwriting their change.

---

## Import what you already have

Pipelines, stores, service accounts and grants you created in the UI can be brought under Terraform with `terraform import`. Most resources import by their ID, and grants import by the path in their `id`:

```bash
terraform import marmot_pipeline.analytics <pipeline-id>

terraform import marmot_asset_iam_binding.orders_editors \
  "asset/<asset-id>/roles/editor"
```

Almost every resource can be imported, and its page on the Registry gives the format. Secret stores and access grants are Marmot Cloud and Marmot Enterprise features. Open source Marmot has no secret store or access policy API, so those resources say so at the top of their page and fail on apply there.

---

## OpenTofu, Pulumi and the operator

The provider is on the OpenTofu Registry with the same source address, so `tofu init` finds it unchanged. Pulumi uses it through the Terraform provider bridge, with `pulumi package add terraform-provider marmotdata/marmot`, in the language your Pulumi program already uses; [Pulumi](/docs/Populating/Pulumi) shows Go and TypeScript. If you'd rather declare pipelines as Kubernetes manifests and let GitOps apply them, the [Kubernetes operator](/docs/Populating/Operator) runs each pipeline as its own Job or CronJob.

---

## How the provider is built

The provider is written in Go on the Terraform Plugin Framework, on top of the [Go SDK](/docs/SDK) we also publish for your own tooling. The framework is what makes ephemeral resources possible. Releases are built for Linux, macOS, Windows and FreeBSD. The documentation is generated from the schema and CI fails if it drifts from the code, and the acceptance tests run against two Terraform versions in CI.

Some of us worked on Terraform at HashiCorp, and we learned a lot there about what makes a provider pleasant to use and what makes one frustrating. Credentials can be ephemeral or write-only, so a key never has to land in state. A policy that changed underneath you is rejected instead of overwritten. Every grant says what it removes on apply, and almost every resource says how to import it. When we add something to the Marmot API, the Terraform resource is part of the same work, not a follow-up ticket. If something doesn't work the way you expect, open an issue on the [provider repository](https://github.com/marmotdata/terraform-provider-marmot). We read all of them.

<CalloutCard
  title="Provider documentation"
  description="Every resource, data source and ephemeral resource, with import syntax and examples, plus guides for access control, keyless pipelines, secret stores and CI."
  href="https://registry.terraform.io/providers/marmotdata/marmot/latest/docs"
  buttonText="Open the docs"
  icon="mdi:book-open-variant"
/>

<CalloutCard
  title="Join the Community"
  description="Get help, share feedback and connect with other Marmot users on Discord."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
