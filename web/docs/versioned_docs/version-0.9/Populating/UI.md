---
sidebar_position: 1
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# UI

Create and manage ingestion pipelines directly from the Marmot web interface.

## Managing Pipelines

The **Runs** page displays all pipelines with their status, schedule, and last run time. You can run, edit, or delete pipelines, and view run history in the **Run History** tab.

<ThemedImg
  lightSrc="/img/ui-runs.png"
  darkSrc="/img/ui-runs-dark.png"
  alt="Runs page"
/>

## Creating a Pipeline

Navigate to **Runs** and click **Create Pipeline** to open the pipeline wizard.

### Step 1: Basic Info

Enter a unique name for your pipeline, e.g. `daily-postgres-sync`.

### Step 2: Choose Plugin

Select the data source you want to discover assets from. Use the search box to quickly find the plugin you need.

<ThemedImg
  lightSrc="/img/ui-choose-plugin.png"
  darkSrc="/img/ui-choose-plugin-dark.png"
  alt="Choose Plugin step"
/>

### Step 3: Configure

Configure the connection settings for your chosen plugin. Options vary by data source but typically include host, authentication, and discovery settings.

<ThemedImg
  lightSrc="/img/ui-configure.png"
  darkSrc="/img/ui-configure-dark.png"
  alt="Configure step"
/>

### Step 4: Schedule

Set a CRON schedule for automated runs, or leave as manual to run on-demand.
