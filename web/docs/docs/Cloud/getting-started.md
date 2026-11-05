---
sidebar_position: 2
title: Get started
description: Sign up for Marmot Cloud, launch an instance and catalog your first source from the UI.
---

import { CliInstall } from '@site/src/components/CliInstall';
import { Steps, Step } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Get started

By the end of this page you'll have a Marmot Cloud instance with your first source cataloged and searchable in **Discover**.

<Steps>
<Step title="Create your account">

Sign up at [cloud.marmotdata.io](https://cloud.marmotdata.io), then choose a plan under **Billing**. **Start on Free** needs no card and gives you 1 instance with room for 500 assets. The [pricing page](/pricing) lists what the paid plans add.

</Step>
<Step title="Launch an instance">

In the console, select **New instance**, enter an **Instance name** and select **Launch instance**. Launching usually takes about a minute and carries on if you leave the page.

The New instance page with a name entered:

<ThemedImg
  lightSrc="/img/cloud-console-new-instance-light.png"
  darkSrc="/img/cloud-console-new-instance-dark.png"
  alt="The console New instance page with Kelp catalog as the instance name, the Europe region selected and the Launch instance button"
  size="panel"
/>

When it's ready, the instance's page shows its **Address** under **Connect**, such as `<instance>.marmotdata.cloud`.

</Step>
<Step title="Sign in">

Under **Admin sign-in** on the instance's page, select **Reveal password**, then open the address and sign in as `admin`. Marmot asks for a new password straight away, and the initial one stops working.

Admin sign-in on the instance's page:

<ThemedImg
  lightSrc="/img/cloud-console-admin-light.png"
  darkSrc="/img/cloud-console-admin-dark.png"
  alt="The Admin sign-in section of an instance page, showing the admin username and the Reveal password button"
  size="dialog"
/>

</Step>
<Step title="Catalog your first source">

In your instance, open **Plugins** and pick your system under **Add a source**:

<ThemedImg
  lightSrc="/img/cloud-add-source-light.png"
  darkSrc="/img/cloud-add-source-dark.png"
  alt="Add a source on the Plugins page, listing plugins grouped by kind with a search box"
/>

1. Enter a **Pipeline name**, such as `warehouse`, and select **Continue**.
2. Fill in the connection details and select **Check and continue**.
3. Leave **Run on a schedule** off for now and select **Create pipeline**.
4. On the pipeline's page, select **Run now**.

Your instance connects to public addresses only. For a database on a private network, run the plugin inside that network with [`marmot plugin`](../Configure/serving-plugins.md). For BigQuery, Glue or Blob Storage, [connect your cloud](Guides/google.md) instead, so the pipeline needs no password.

</Step>
<Step title="Find it in Discover">

When the run finishes, open **Discover**. Your source's tables, topics or files are listed there, alongside data products, glossary terms and teams in one search:

<ThemedImg
  lightSrc="/img/cloud-discover-light.png"
  darkSrc="/img/cloud-discover-dark.png"
  alt="Discover in a Marmot Cloud instance, with filters for kind, type and provider beside a list of cataloged assets"
  size="window"
/>

</Step>
</Steps>

## Optional: work from your terminal

Everything above can also be done from the CLI or Terraform. Neither is needed to use Marmot Cloud.

Install the CLI and sign in. Your browser opens, and the CLI keeps a token for 24 hours:

<CliInstall />

```bash
marmot login <instance>.marmotdata.cloud
marmot assets list
```

The Terraform provider signs in with the same CLI session, so you don't need an API key on your own machine:

```hcl
terraform {
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

In CI, give Terraform a [service account of its own](service-accounts.md#run-terraform-in-ci).

## Next steps

- [Connect your cloud](Guides/google.md): catalog Google Cloud, AWS or Azure with no stored credential.
- [Set up single sign-on](single-sign-on.md): let your team sign in through your identity provider.
- [Access control](access-control.md): decide who sees what.
- [Connect an agent](/docs/MCP): point Claude, Cursor or your own agent at `https://<instance>.marmotdata.cloud/api/v1/mcp`.
