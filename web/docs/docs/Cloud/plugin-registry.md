---
sidebar_position: 8
title: Publish private plugins
description: Push your own Marmot plugins to your instance's private registry and run them like any core plugin.
---

import { Steps, Step } from '@site/src/components/Steps';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Publish private plugins

Push a plugin you've built to your instance's own registry and run it like any core plugin, with the same config checks, pipelines and run history. Nobody outside your instance can see it. Every plan except Free includes a registry.

Writing the plugin is covered in [creating plugins](/docs/Develop/creating-plugins). Instances run on `linux/arm64`, so build for that platform.

## Before you start

- You can sign in to your instance with the Marmot CLI.
- You have [ORAS](https://oras.land) installed, with `oras manifest index create`.
- You have `plugin.publisher` across the whole catalog.
- Your instance has a registry. If `curl https://<instance>.marmotdata.cloud/v2/` answers `404` with `This server does not serve a plugin registry`, it doesn't: the instance is on the Free plan.

## Push a plugin

<Steps>
<Step title="Sign in">

```bash
marmot login https://<instance>.marmotdata.cloud
```

Signing in also registers your token with the Docker credential store, which `docker`, `crane` and `oras` read, so you don't need a separate registry login.

</Step>
<Step title="Push">

Push the `linux/arm64` build under its own tag, then tie it to a version with an OCI index. The index's tag is the version, and Marmot only records a version when the index arrives:

```bash
REPO=<instance>.marmotdata.cloud/plugins/internal-scheduler
gzip -c dist/marmot-plugin-internal-scheduler > marmot-plugin-internal-scheduler

oras push "$REPO:0.1.0-linux-arm64" \
  --artifact-type application/vnd.marmot.plugin.v1 \
  --artifact-platform linux/arm64 \
  marmot-plugin-internal-scheduler:application/vnd.marmot.plugin.v1+gzip

oras manifest index create "$REPO:0.1.0" 0.1.0-linux-arm64
```

To ship more platforms, push one manifest per platform and list them all in the index, as [creating plugins](../Develop/creating-plugins.md#6-publish-the-plugin) shows.

</Step>
</Steps>

## Names and versions

| Rule | Limit |
| --- | --- |
| Plugin names | Lowercase letters, digits and dashes, up to 63 characters, not the name of a core plugin |
| Upload size | 512 MiB each |
| Versions | Can't change once pushed |

Pushing different contents as `0.1.0` is rejected with `internal-scheduler 0.1.0 is already pushed and versions are immutable; push it as a new version`, so cut `0.1.1`.

## Use it

Marmot loads the plugin as soon as the index push is accepted. Create a pipeline that uses it, as you would for any core plugin:


<Tabs groupId="method" queryString>
  <TabItem value="ui" label="UI" default>

1. Open **Plugins** and select your plugin under **Add a source**.
2. Enter a **Pipeline name** and select **Continue**.
3. Fill in the plugin's configuration and select **Check and continue**.
4. Set a schedule and select **Create pipeline**.

  </TabItem>
  <TabItem value="terraform" label="Terraform">

```hcl
resource "marmot_pipeline" "internal_jobs" {
  name            = "internal-jobs"
  plugin_id       = "internal-scheduler"
  cron_expression = "0 */4 * * *"

  config = jsonencode({
    endpoint = "https://scheduler.example.com"
  })
}
```

  </TabItem>
</Tabs>

Credentials go through a [secret store](secret-stores.md), as they do for a core plugin.

## Push from CI

A version pushed from a laptop is one nobody can reproduce, so push from your release workflow:

1. [Create a service account](service-accounts.md) for the workflow and grant it `plugin.publisher` across the whole catalog. That role can push plugins and see which are installed, and nothing else.
2. Store its API key as a secret in your CI system, such as `MARMOT_API_KEY`.
3. In the workflow, sign in with the key as the password and any username:

```bash
echo "$MARMOT_API_KEY" | \
  oras login <instance>.marmotdata.cloud -u ci --password-stdin
```

## Pin a version

A pipeline either runs the newest version available when it runs or stays on one pushed version, and a source that answers queries uses the same version for them. Set **Version** when you create the pipeline, or under **Edit pipeline** later. Leave pipelines on the latest version while you're iterating, then pin them once people rely on them, so a routine push can't change the code behind a production pipeline.

## Next steps

- [Creating plugins](/docs/Develop/creating-plugins): the plugin SDK, the config spec, asset schemas and the local development loop.
- [Security and isolation](security.md): how instances are kept apart and what Marmot stores.
