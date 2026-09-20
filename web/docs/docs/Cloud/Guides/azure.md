---
sidebar_position: 3
title: Azure
description: Your first pipeline on Marmot Cloud, cataloging Blob Storage with no client secret.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { Steps, Step } from '@site/src/components/Steps';

# Azure

This guide takes you from an empty instance to a pipeline that catalogs a storage account every six hours. The pipeline authenticates to Entra ID as itself, so there is no client secret to create, store or rotate.

You need a Marmot Cloud instance, written as `https://acme.marmotdata.cloud` throughout, an Entra ID tenant where you can create app registrations, a subscription where you can assign roles, and Terraform with the `marmot`, `azuread` and `azurerm` providers. If you have not signed in from the CLI yet, do [getting started](../getting-started.md) first.

## How it works

Your instance is an OIDC issuer. A federated identity credential on an app registration tells Entra ID that a token from your instance, for a given pipeline, may act as that application. The application never has a password or certificate. Each pipeline gets its own app registration.

## Your first pipeline

<Steps>
<Step title="Configure the providers">

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    marmot  = { source = "marmotdata/marmot" }
    azuread = { source = "hashicorp/azuread" }
    azurerm = { source = "hashicorp/azurerm" }
  }
}

provider "marmot" {
  host = "https://acme.marmotdata.cloud"
}

provider "azurerm" {
  features {}
}

data "azurerm_client_config" "current" {}
```

</Step>
<Step title="Create an app registration for the pipeline">

No application password and no certificate. The federated credential added in a later step is the application's only way to authenticate.

```hcl
resource "azuread_application" "marmot_blob" {
  display_name = "marmot-blob-pipeline"
}

resource "azuread_service_principal" "marmot_blob" {
  client_id = azuread_application.marmot_blob.client_id
}
```

</Step>
<Step title="Declare the pipeline">

Supplying the tenant and client id with no account key is what tells the plugin to authenticate as the pipeline.

```hcl
resource "marmot_pipeline" "blobs" {
  name      = "blobs"
  plugin_id = "azureblob"

  config = jsonencode({
    account_name = "acmeprod"
    tenant_id    = data.azurerm_client_config.current.tenant_id
    client_id    = azuread_application.marmot_blob.client_id
  })

  cron_expression = "0 */6 * * *"
}
```

</Step>
<Step title="Trust the pipeline and grant it access">

Read the issuer and subject from the pipeline resource. Entra ID matches them exactly, and when one is wrong the error does not say which.

```hcl
resource "azuread_application_federated_identity_credential" "marmot_blob" {
  application_id = azuread_application.marmot_blob.id
  display_name   = "marmot-blob-pipeline"
  issuer         = marmot_pipeline.blobs.issuer
  subject        = marmot_pipeline.blobs.subject
  audiences      = ["api://AzureADTokenExchange"]
}

data "azurerm_storage_account" "prod" {
  name                = "acmeprod"
  resource_group_name = "acme-prod"
}

resource "azurerm_role_assignment" "marmot_reads_blobs" {
  scope                = data.azurerm_storage_account.prod.id
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = azuread_service_principal.marmot_blob.object_id
}
```

</Step>
<Step title="Apply and run">

```bash
terraform apply
marmot runs list
```

The pipeline runs on its next tick, or now from the Pipelines page in your instance. When the run completes, your containers and blobs are in the catalog.

</Step>
</Steps>

Every other Azure plugin works the same way: an app registration per pipeline, a federated credential, and a read-only role assignment. A user-assigned managed identity works in place of the app registration. Each plugin's page on the [plugin registry](https://plugins.marmotdata.io) names its role.

## When a source needs a password

An Azure Database for PostgreSQL or self-managed database cannot federate, so its pipeline needs a real password. Keep it in Key Vault and let Marmot read it at run time through a secret store. The store gets an app registration of its own.

```hcl
resource "azuread_application" "marmot_secrets" {
  display_name = "marmot-secret-store"
}

resource "azuread_service_principal" "marmot_secrets" {
  client_id = azuread_application.marmot_secrets.client_id
}

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

data "azurerm_key_vault" "prod" {
  name                = "acme-prod"
  resource_group_name = "acme-prod"
}

resource "azurerm_role_assignment" "marmot_reads_secrets" {
  scope                = data.azurerm_key_vault.prod.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = azuread_service_principal.marmot_secrets.object_id
}

resource "marmot_secret_store_azure_secret" "orders_db_password" {
  store     = marmot_secret_store_azure.prod.id
  vault_uri = data.azurerm_key_vault.prod.vault_uri
  name      = "orders-db-password"
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
    password = marmot_secret_store_azure_secret.orders_db_password.id
  }

  cron_expression = "0 * * * *"
}
```

Key Vault Secrets User reads secret values and nothing else. No version is pinned, so creating a new version in Key Vault is the whole rotation. [Secret stores](../secret-stores.md) covers the model.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| No matching federated identity record | Issuer, subject or audience differs from what Marmot sent. Re-read them from the resource. |
| Authenticated, request refused | Role assignment missing, at the wrong scope, or not yet propagated. Entra ID can take a few minutes. |
| Plugin asks for an account key | Tenant id or client id is missing from the config. |

<CalloutCard
  title="Next: decide who sees what"
  description="Grant roles per asset, data product, glossary term and secret store."
  docId="Cloud/access-control"
  buttonText="Access control"
  icon="mdi:shield-key"
/>
