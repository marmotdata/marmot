---
sidebar_position: 1
title: Pipelines
description: A plugin pointed at a source, run by your Marmot server on a schedule. Declare it in Terraform or create it in the UI.
---

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';

# Pipelines

A pipeline is a plugin pointed at a source, running on a schedule. You describe the connection once and the plugin discovers what is there on every run, so the catalog stays in step with the system it describes. The Marmot server runs the plugin, so the server needs network access to the source.

## Declare a pipeline

Pipelines are declared with the [Terraform provider](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs), which keeps them next to the infrastructure they catalog and puts every change through review.

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
  host = "https://marmot.acme.internal"
}

resource "marmot_pipeline" "orders" {
  name      = "orders"
  plugin_id = "postgresql"

  config = jsonencode({
    host     = "orders-db.acme.internal"
    database = "orders"
    user     = "marmot"
    password = var.orders_db_password
  })

  cron_expression = "0 * * * *"
}
```

The provider uses your `marmot login` session on your own machine and an API key in CI. See [Terraform](Terraform.md) for provider configuration.

| Field | Meaning |
| --- | --- |
| `name` | Identifies the pipeline. |
| `plugin_id` | Which plugin runs. See the [plugin registry](https://plugins.marmotdata.io). |
| `config` | Plugin settings. Validated against the plugin when you apply. |
| `cron_expression` | How often it runs. |
| `enabled` | Set to false to pause. |

```bash
terraform apply
marmot runs list
```

The pipeline runs on its next cron tick, or immediately from the Runs page.

## Credentials

On open source Marmot the credential is part of the config. It is encrypted at rest on the server and never returned by the API. Keep the source value in your secret manager and pass it through a Terraform variable, as above, so it is not written into the configuration itself.

On Marmot Cloud a pipeline holds no credential: it authenticates to Google Cloud, AWS or Azure as itself, or reads a password from your secret manager just before each run.

<CalloutCard
  title="Pipelines on Marmot Cloud"
  description="Keyless pipelines and secret stores, so no credential is stored in Marmot or in Terraform state."
  docId="Cloud/pipelines"
  buttonText="Marmot Cloud"
  icon="mdi:key-remove"
/>

## Schedules

Match the schedule to how often the source changes. Spread pipelines across the hour so they do not all start at once. Pause with `enabled = false` rather than deleting, so the run history stays. Use the plugin's include patterns to catalog only the schemas people ask about.

## Runs

```bash
marmot runs list
marmot runs get <id>
```

The Runs page shows every pipeline with its status and next run, and a log for each run. Each pipeline resource also reports its last and next run, so a Terraform output can serve as a health check.

## Creating pipelines in the UI

The Runs page has a wizard that creates the same pipeline from a form: name, plugin, configuration and schedule. It is useful for trying a plugin before writing it down. A pipeline that people rely on belongs in Terraform, so its definition has one owner. See [UI](UI.md).

## Running plugins outside the server

The CLI and the Kubernetes operator run plugins on the machine or cluster where they are invoked and push the results to Marmot. Use them when the server cannot reach the source, or when you want each source's credentials confined to its own runner.

<DocCardGrid>
  <DocCard
    title="CLI"
    description="Run plugins from any machine or CI job with a YAML configuration"
    docId="Populating/CLI"
    icon="mdi:console"
  />
  <DocCard
    title="Kubernetes operator"
    description="Run each pipeline as its own Job or CronJob in your cluster"
    docId="Populating/Operator"
    icon="mdi:kubernetes"
  />
</DocCardGrid>
