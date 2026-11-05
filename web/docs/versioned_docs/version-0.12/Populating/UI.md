---
sidebar_position: 2
title: Create a pipeline in the UI
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Create a pipeline in the UI

The **Plugins** page creates and runs [pipelines](Pipelines.md) from a form, which is the quickest way to try a plugin. Every pipeline is listed there with its last run, recent runs, schedule and the assets it catalogs:

<ThemedImg
  lightSrc="/img/ui-runs.png"
  darkSrc="/img/ui-runs-dark.png"
  alt="The Plugins page listing pipelines with their last run, recent runs, schedule and asset count, and Run now on each row"
  size="panel"
/>

## Create a pipeline

1. On **Plugins**, select **Add plugin**, or pick a plugin under **Add a source**.
2. Under **Choose plugin**, search for your source, select it and select **Continue**.
3. Under **Basic info**, enter a **Pipeline name**, such as `marketplace-postgres`, and select **Continue**.
4. Under **Configure**, fill in the connection details. **Test connection** lists what the plugin can see without changing the catalog. For a plugin marked **Supports querying**, turn on **Let agents and people query this source** if it should answer read-only queries. Select **Check and continue**.
5. Under **Schedule and filters**, turn on **Run on a schedule** if it should run by itself, choose when the **First run** happens and select **Create pipeline**.

The plugin chosen:

<ThemedImg
  lightSrc="/img/ui-choose-plugin.png"
  darkSrc="/img/ui-choose-plugin-dark.png"
  alt="The Choose plugin step of New pipeline, filtered to PostgreSQL"
  size="panel"
/>

The **Configure** step for a PostgreSQL source:

<ThemedImg
  lightSrc="/img/ui-configure.png"
  darkSrc="/img/ui-configure-dark.png"
  alt="The Configure step for a PostgreSQL pipeline named marketplace-postgres, with host db.internal, user marmot_reader and the discovery switches"
  size="panel"
/>

Options differ by plugin. Each plugin's page in the [plugin registry](https://plugins.marmotdata.io) lists them.

## Run and edit pipelines

**Run now** on a pipeline's row starts it straight away. The menu on the row has **View runs**, for its run history and the log of each run, **Edit pipeline** and delete.

## Next steps

- [Pipelines](Pipelines.md): how pipelines run and how to declare them in Terraform.
- [From the CLI](CLI.md): run a plugin from your own machine or CI and push the results.
