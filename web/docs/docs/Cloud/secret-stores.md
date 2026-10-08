---
sidebar_position: 6
title: Secret stores
description: Point Marmot at Secret Manager, Secrets Manager, Key Vault or Vault through a federated identity. Marmot stores the address of a secret, never its value.
---

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Tabs, TabPanel } from '@site/src/components/Steps';

# Secret stores

A pipeline against a database needs a password. Marmot never stores it. It stores where the password lives in your secret manager and reads the value just before each run. Marmot authenticates to your secret manager with a short-lived token it mints itself, so there is no credential for reading credentials either.

## Stores and secrets

A store is a connection to one secret manager. Keep one per account or environment, so production and staging are two stores.

A secret is the address of one value inside a store. It holds no value, so it is safe in Terraform, in git and in plan output.

| Backend | Store | Secret |
| --- | --- | --- |
| Google Secret Manager | `marmot_secret_store_google` | `marmot_secret_store_google_secret` |
| AWS Secrets Manager | `marmot_secret_store_aws` | `marmot_secret_store_aws_secret` |
| Azure Key Vault | `marmot_secret_store_azure` | `marmot_secret_store_azure_secret` |
| HashiCorp Vault | `marmot_secret_store_vault` | `marmot_secret_store_vault_secret` |

## Register a store

Each store authenticates to its backend as an identity named after the store. Grant that identity on the secrets it should read and nothing else. The guide for your cloud builds the trust relationship end to end.

<Tabs items={[
{ label: "Google Cloud", value: "google", icon: "mdi:google-cloud" },
{ label: "AWS", value: "aws", icon: "mdi:aws" },
{ label: "Azure", value: "azure", icon: "mdi:microsoft-azure" },
{ label: "Vault", value: "vault", icon: "simple-icons:vault" }
]} groupId="cloud">
<TabPanel>

```hcl
resource "marmot_secret_store_google" "prod" {
  name                       = "gcp-prod"
  workload_identity_provider = google_iam_workload_identity_pool_provider.marmot.name
}

resource "google_secret_manager_secret_iam_member" "marmot_reads_orders_password" {
  secret_id = google_secret_manager_secret.orders_db_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.marmot.name}/subject/${marmot_secret_store_google.prod.subject}"
}
```

</TabPanel>
<TabPanel>

The full trust policy is in the [AWS guide](Guides/aws.md#when-a-source-needs-a-password).

```hcl
locals {
  secret_store_name = "aws-prod"
}

resource "aws_iam_role" "marmot_store" {
  name               = "marmot-secret-store"
  assume_role_policy = data.aws_iam_policy_document.marmot_store_trust.json
}

resource "marmot_secret_store_aws" "prod" {
  name     = local.secret_store_name
  role_arn = aws_iam_role.marmot_store.arn
}
```

</TabPanel>
<TabPanel>

```hcl
resource "marmot_secret_store_azure" "prod" {
  name      = "azure-prod"
  tenant_id = data.azurerm_client_config.current.tenant_id
  client_id = azuread_application.marmot_secrets.client_id
}

resource "azuread_application_federated_identity_credential" "marmot_secrets" {
  application_id = azuread_application.marmot_secrets.id
  display_name   = "marmot-secret-store"
  issuer         = marmot_secret_store_azure.prod.issuer
  subject        = marmot_secret_store_azure.prod.subject
  audiences      = [marmot_secret_store_azure.prod.audience]
}
```

</TabPanel>
<TabPanel>

```hcl
resource "marmot_secret_store_vault" "prod" {
  name    = "vault-prod"
  address = "https://vault.acme.internal"
  role    = "marmot"
}

resource "vault_jwt_auth_backend" "marmot" {
  path               = "jwt"
  oidc_discovery_url = marmot_secret_store_vault.prod.issuer
}

resource "vault_policy" "marmot" {
  name = "marmot"

  policy = <<-EOT
    path "secret/data/orders/*" {
      capabilities = ["read"]
    }
  EOT
}

resource "vault_jwt_auth_backend_role" "marmot" {
  backend         = vault_jwt_auth_backend.marmot.path
  role_name       = "marmot"
  role_type       = "jwt"
  user_claim      = "sub"
  bound_subject   = marmot_secret_store_vault.prod.subject
  bound_audiences = [marmot_secret_store_vault.prod.audience]
  token_policies  = [vault_policy.marmot.name]
}
```

Vault Enterprise namespaces and private certificate authorities are supported through the namespace and CA certificate fields.

</TabPanel>
</Tabs>

The store's name is its identity. Renaming it replaces the store and breaks every grant that named it, so name it after the account it reaches, not the first thing you use it for.

<DocCardGrid>
  <DocCard
    title="Google Cloud setup"
    description="A store reading from Secret Manager, in the Google Cloud guide"
    docId="Cloud/Guides/google"
    icon="mdi:google-cloud"
  />
  <DocCard
    title="AWS setup"
    description="A store reading from Secrets Manager, in the AWS guide"
    docId="Cloud/Guides/aws"
    icon="mdi:aws"
  />
  <DocCard
    title="Azure setup"
    description="A store reading from Key Vault, in the Azure guide"
    docId="Cloud/Guides/azure"
    icon="mdi:microsoft-azure"
  />
</DocCardGrid>

## Register a secret

Declare the secret and the Marmot reference to it together, so they cannot drift.

```hcl
resource "google_secret_manager_secret" "orders_db_password" {
  secret_id = "orders-db-password"

  replication {
    auto {}
  }
}

resource "marmot_secret_store_google_secret" "orders_db_password" {
  store     = marmot_secret_store_google.prod.id
  project   = google_secret_manager_secret.orders_db_password.project
  secret_id = google_secret_manager_secret.orders_db_password.secret_id
}
```

| Backend | Address fields |
| --- | --- |
| Google | `project`, `secret_id` |
| AWS | `secret_id` as an ARN, or `region` and `secret_id` |
| Azure | `vault_uri`, `name` |
| Vault | `mount`, `name`, `key` |

Leave the version unset. Marmot then reads the current value on every run, and rotation happens entirely in your secret manager.

## Use a secret

A [pipeline](pipelines.md#sources-that-need-a-password) names the secret in its secrets map, and Marmot fills the value in before each run. The value exists in memory for the run and is then discarded.

A principal with the reader role can also read a value directly. This is the only path in Marmot that returns plaintext, and it is how an agent gets a credential without that credential appearing in any configuration file.

## Access control

| Role | May | Binds on |
| --- | --- | --- |
| `secretStore.viewer` | See a store and its secrets. No values. | organization, secretStore |
| `secretStore.user` | Register secrets and attach them to pipelines. No values. | organization, secretStore |
| `secretStore.reader` | Read values. | organization, secretStore |
| `secretStore.admin` | Create, edit and delete stores. | organization |

The boundary that matters is between user and reader. Writing a pipeline means saying where its password comes from, not seeing it, so almost nobody who writes pipelines needs the reader role.

```hcl
resource "marmot_secret_store_iam_member" "data_engineering_uses_prod" {
  secret_store_id = marmot_secret_store_google.prod.id
  role            = "secretStore.user"
  member          = "group:${marmot_team.data_engineering.id}"
}

resource "marmot_secret_store_iam_member" "copilot_reads_prod" {
  secret_store_id = marmot_secret_store_google.prod.id
  role            = "secretStore.reader"
  member          = "serviceAccount:${marmot_service_account.copilot.id}"
}
```

## Notes

- A store in use cannot be deleted. Remove the pipeline references first.
- Validate a new store from its page in your instance. It performs a real token exchange and reports the result, which beats finding a mistyped subject at the next scheduled run.
- Values are never persisted. There is no cache to expire after a rotation.

<CalloutCard
  title="Or skip the secret entirely"
  description="For AWS, Google Cloud and Azure sources, a pipeline can present its own identity and use no credential at all."
  docId="Cloud/pipelines"
  buttonText="Keyless pipelines"
  icon="mdi:key-remove"
/>
