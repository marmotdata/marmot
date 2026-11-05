---
sidebar_position: 1
title: Pipelines
description: A plugin pointed at a source, run by your Marmot server on a schedule. Create it in the UI or declare it in Terraform.
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Pipelines

A pipeline is a plugin pointed at a source, running on a schedule. You describe the connection once and the plugin discovers what is there on every run, so the catalog stays in step with the system it describes. The Marmot server runs the plugin, so the server needs network access to the source.

## Create a pipeline

<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

On **Plugins**, select **Add plugin**, choose the plugin, name the pipeline, fill in the connection and choose a schedule. [Create a pipeline in the UI](UI.md) walks through each step.

  </TabItem>
  <TabItem value="terraform" label="Terraform">

The [Terraform provider](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs) keeps a pipeline next to the infrastructure it catalogs and puts every change through review:

```hcl
terraform {
  required_providers {
    marmot = {
      source = "marmotdata/marmot"
    }
  }
}

provider "marmot" {
  host = "https://marmot.example.com"
}

resource "marmot_pipeline" "marketplace" {
  name      = "marketplace-postgres"
  plugin_id = "postgresql"

  config = jsonencode({
    host     = "db.internal"
    user     = "marmot_reader"
    password = var.marketplace_db_password
  })

  cron_expression = "0 3 * * *"
}
```

| Field | Meaning |
| --- | --- |
| `name` | Identifies the pipeline. |
| `plugin_id` | Which plugin runs. See the [plugin registry](https://plugins.marmotdata.io). |
| `config` | Plugin settings, validated against the plugin when you apply. |
| `cron_expression` | How often it runs. |
| `enabled` | Set to false to pause. |

The provider uses your `marmot login` session on your own machine and an API key in CI. See [Terraform](Terraform.md) for provider configuration.

  </TabItem>
</Tabs>

## Check that it worked

The pipeline runs on its next scheduled run, or straight away with **Run now** on **Plugins**. Its row shows the last run and recent runs, and **View runs** shows the log of each one. From a terminal:

```bash
marmot runs list --pipelines marketplace-postgres
```

## Credentials

On open source Marmot the credential is part of the config. It's encrypted at rest on the server and never returned by the API. In Terraform, keep the value in your secret manager and pass it through a variable, as above, so it isn't written into the configuration itself.

On Marmot Cloud a pipeline holds no credential: it authenticates to Google Cloud, AWS or Azure as itself, or reads a password from your secret manager shortly before each run. See [pipelines on Marmot Cloud](../Cloud/pipelines.md).

## Schedules

Match the schedule to how often the source changes, and spread pipelines across the hour so they don't all start at once. To stop a pipeline running for a while, pause it (**Pause the schedule** when editing it, or `enabled = false` in Terraform) rather than deleting it, so the run history stays. Use the plugin's include patterns to catalog only the schemas people ask about.

## Running plugins outside the server

[The CLI](CLI.md) and [the Kubernetes operator](Operator.md) run plugins on the machine or cluster where they are invoked and push the results to Marmot. Use them when the server can't reach the source, or when you want each source's credentials confined to its own runner.

## Next steps

- [Create a pipeline in the UI](UI.md): every step of the form.
- [From the CLI](CLI.md): run a plugin from your own machine or CI.
- [Plugin registry](https://plugins.marmotdata.io): the options for every plugin.
