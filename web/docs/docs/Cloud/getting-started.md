---
sidebar_position: 2
title: Getting started
description: Sign up for Marmot Cloud, launch an instance, sign in from the CLI and start a Terraform configuration.
---

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { CliInstall } from '@site/src/components/CliInstall';
import { Steps, Step, TipBox } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Getting started

This guide gets you a running instance, a terminal signed in to it, and an empty Terraform configuration that can reach it. It takes about five minutes.

<Steps>
<Step title="Create your account">

Sign up at [cloud.marmotdata.io](https://cloud.marmotdata.io) and pick a plan. Free needs no card and has no end date: one Standard instance with a 500-asset catalog, enough to point Marmot at a system you actually run.

</Step>
<Step title="Launch an instance">

Launching takes about a minute and continues if you close the page. You end with an address like `https://acme.marmotdata.cloud`. That is your instance, and almost everything from here happens there rather than in the console.

</Step>
<Step title="Sign in to your instance">

Reveal the initial password next to the `admin` username in the console and sign in at your instance URL. Marmot asks for a new password immediately, and the generated one stops working.

<TipBox variant="success" title="Do this before anyone else signs in">
Configure [single sign-on](single-sign-on.md) now, while the instance has one user. Retrofitting it onto an instance full of local accounts means reconciling each one by hand.
</TipBox>

</Step>
<Step title="Connect your terminal">

<CliInstall />

```bash
marmot login https://acme.marmotdata.cloud
marmot assets list
```

Your browser opens, you sign in, and the CLI stores a 24-hour token. An empty asset list is correct on a fresh instance.

Once a pipeline has run, the same catalog is on Discover: assets, data products, glossary terms and teams behind one search.

<ThemedImg
  lightSrc="/img/cloud-discover-light.png"
  darkSrc="/img/cloud-discover-dark.png"
  alt="The Discover page with filters for kind, type and provider beside a list of catalogued assets"
/>

</Step>
<Step title="Start a Terraform configuration">

Everything you do to the instance from here is declared in Terraform. Create a repository and start with the provider block alone:

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

```bash
terraform init
terraform plan
```

The provider authenticates with your CLI session, so there is no API key to create. An empty plan confirms it can reach the instance. When the configuration moves to CI it gets a [service account of its own](access-control.md#run-terraform-in-ci).

</Step>
</Steps>

## Your first pipeline

Pick the cloud your data lives in. Each guide walks you from the empty configuration above to a running pipeline that catalogs a native source with no credential at all, then shows how to add a source that needs a password.

<DocCardGrid>
  <DocCard
    title="Google Cloud"
    description="Catalog BigQuery with Workload Identity Federation"
    docId="Cloud/Guides/google"
    icon="mdi:google-cloud"
  />
  <DocCard
    title="AWS"
    description="Catalog the Glue Data Catalog with an IAM role"
    docId="Cloud/Guides/aws"
    icon="mdi:aws"
  />
  <DocCard
    title="Azure"
    description="Catalog Blob Storage with a federated app registration"
    docId="Cloud/Guides/azure"
    icon="mdi:microsoft-azure"
  />
  <DocCard
    title="Somewhere else"
    description="A pipeline that reads its password from your secret manager at run time"
    docId="Cloud/pipelines"
    icon="mdi:pipe"
  />
</DocCardGrid>

## Next steps

<DocCardGrid>
  <DocCard
    title="Lock down access"
    description="Grant roles per asset, data product, glossary term and secret store"
    docId="Cloud/access-control"
    icon="mdi:shield-key"
  />
  <DocCard
    title="Connect your agents"
    description="Point Claude, Cursor or your own agent at your instance's MCP endpoint"
    docId="MCP/index"
    icon="mdi:robot"
  />
  <DocCard
    title="Manage your instance"
    description="Sizes, stopping and starting, and your own domain"
    docId="Cloud/instances"
    icon="mdi:server"
  />
  <DocCard
    title="Plans and billing"
    description="What each plan includes and how sizes are billed"
    docId="Cloud/plans"
    icon="mdi:chart-box"
  />
</DocCardGrid>
