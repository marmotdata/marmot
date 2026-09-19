---
sidebar_position: 1
title: Marmot Cloud
description: Marmot, run for you. A managed instance on its own hostname, with fine-grained access control, federated secret stores and keyless pipelines.
---

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

# Marmot Cloud

Marmot Cloud is the hosted version of Marmot. You get the same catalog described in the rest of these docs, on a hostname of your own, with the Postgres cluster, upgrades, backups and TLS managed for you. The data model, the query language, the plugins and the MCP server are unchanged, so everything else in this documentation applies as written.

Marmot Cloud adds three things that only make sense when someone else runs the server: access control per resource, secret stores that read from the vault you already run, and a workload identity issuer that lets pipelines authenticate to your cloud with no stored credential. Everything in this section applies to Marmot Cloud and Marmot Enterprise. Where a page covers something open source Marmot also has, such as pipelines, it says so.

<CalloutCard
  title="Launch your first instance"
  description="Sign in, pick a region and a size, and have a Marmot instance answering requests in about a minute."
  docId="Cloud/getting-started"
  buttonText="Get started"
  icon="mdi:rocket-launch"
/>

## Features

<FeatureGrid>
  <FeatureCard
    title="A managed instance"
    description="Your own hostname, an isolated Postgres cluster with backups, and TLS."
    docId="Cloud/instances"
    icon="mdi:server"
  />
  <FeatureCard
    title="Fine-grained access control"
    description="Grant roles on the whole catalog, a data product, one asset or a glossary term."
    docId="Cloud/access-control"
    icon="mdi:shield-key"
  />
  <FeatureCard
    title="Federated secret stores"
    description="Marmot stores the address of a secret in your own vault, never the value."
    docId="Cloud/secret-stores"
    icon="mdi:safe"
  />
  <FeatureCard
    title="Keyless pipelines"
    description="A pipeline presents its own short-lived identity to AWS, Google Cloud or Azure."
    docId="Cloud/pipelines"
    icon="mdi:key-remove"
  />
  <FeatureCard
    title="Single sign-on"
    description="Connect any OIDC provider and map its groups onto Marmot roles."
    docId="Cloud/single-sign-on"
    icon="mdi:login-variant"
  />
  <FeatureCard
    title="A private plugin registry"
    description="Push your own plugins to your instance and run them like any core plugin."
    docId="Cloud/plugin-registry"
    icon="mdi:package-variant-closed"
  />
</FeatureGrid>

## The console and your instance

Marmot Cloud has two addresses.

The **console** at `cloud.marmotdata.io` is the control plane for your account: your plan, your billing, and the instances you own. You launch, stop and delete instances there, and pick up the initial administrator password. It knows nothing about the contents of your catalog and has no API.

Your **instance** is the Marmot server itself, at an address like `https://acme.marmotdata.cloud`. Every asset, pipeline, service account and access grant lives there, along with the [REST API](/docs/api-reference), the [MCP endpoint](/docs/MCP) and the OIDC documents behind [keyless authentication](pipelines.md#how-keyless-pipelines-work).

You touch the console to create an instance and to change plan. Everything else happens against the instance.

## Manage it with Terraform

Every change to an instance in these docs is Terraform, and that is how we recommend running one. Pipelines, secret stores, service accounts and access grants are resources in the [Marmot provider](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs). The cloud-side half of each, the IAM role or the federated credential, is a resource in your cloud provider. Declaring both in one configuration keeps a pipeline's identity and the trust that admits it from drifting apart.

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
  host = "https://acme.marmotdata.cloud"
}
```

On your own machine the provider uses your `marmot login` session. In CI it runs as a [service account](access-control.md#run-terraform-in-ci). The UI is for reading: run history, the access panel on an asset, validating a secret store.

## Differences from self-hosting

The catalog, the query language, the plugins, the MCP server and the SDKs are identical, and a Cloud instance can be swapped for a self-hosted one without rewriting ingestion. Three subsystems exist only on Marmot Cloud and Marmot Enterprise.

| | What it adds |
| --- | --- |
| [Access control](access-control.md) | Roles granted on one asset, data product, glossary term or secret store, not only across the instance. This is what lets an agent hold a credential scoped to the assets it should see. |
| [Secret stores](secret-stores.md) | Marmot stores the address of a credential in your secret manager and resolves it before each run. Nothing is copied into Marmot or Terraform state. |
| [Workload identity](pipelines.md#how-keyless-pipelines-work) | Your instance mints a short-lived token per pipeline and per store. Your cloud exchanges it for access. There is no key to store, rotate or leak. |

## Next steps

<DocCardGrid>
  <DocCard
    title="Getting started"
    description="Launch an instance, sign in with the CLI, and start a Terraform configuration"
    docId="Cloud/getting-started"
    icon="mdi:flag-checkered"
  />
  <DocCard
    title="Connect your cloud"
    description="Your first pipeline on Google Cloud, AWS or Azure, with no credential"
    docId="Cloud/Guides/google"
    icon="mdi:cloud-sync"
  />
  <DocCard
    title="Plans and billing"
    description="What each plan includes and how sizes are billed"
    docId="Cloud/plans"
    icon="mdi:chart-box"
  />
  <DocCard
    title="Security and isolation"
    description="How instances are kept apart and how your data is protected"
    docId="Cloud/security"
    icon="mdi:shield-lock"
  />
</DocCardGrid>
