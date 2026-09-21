---
sidebar_position: 8
title: Private plugin registry
description: Push your own Marmot plugins to your instance and run them like any core plugin.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { Steps, Step, TipBox } from '@site/src/components/Steps';

# Private plugin registry

Every Marmot Cloud instance from the Team plan up serves an OCI registry of its own. Build a plugin against the same SDK the core plugins use, push it to your instance, and it runs like any built-in plugin, with the same config validation, pipelines and run history. Nobody else can see it.

Writing the plugin is covered in [creating plugins](/docs/Develop/creating-plugins). One requirement is specific to Cloud: instances run on `linux/arm64`, so build for that platform.

<TipBox variant="info" title="Team and above">
If `curl https://<your-host>/v2/` answers `404`, the registry is not enabled on your instance. [Ask support](mailto:support@marmotdata.io).
</TipBox>

## Push a plugin

<Steps>
<Step title="Sign in">

```bash
marmot login https://acme.marmotdata.cloud
```

Signing in also registers your token with `docker`, `crane` and `oras` for that host, so no separate registry login is needed.

</Step>
<Step title="Push">

The repository is `<your-host>/plugins/<name>` and the tag is the version. Build the binary for `linux/arm64`, gzip it, and push it:

```bash
gzip -c dist/marmot-plugin-internal-scheduler > marmot-plugin-internal-scheduler
oras push acme.marmotdata.cloud/plugins/internal-scheduler:0.1.0 \
  --artifact-type application/vnd.marmot.plugin.v1 \
  --artifact-platform linux/arm64 \
  marmot-plugin-internal-scheduler:application/vnd.marmot.plugin.v1+gzip
```

To ship more than one platform, push one manifest per platform and tie them with an index, as [creating plugins](../Develop/creating-plugins.md#6-publish-the-plugin) shows.

Names are lowercase with dashes and cannot shadow a core plugin. Versions are immutable: pushing `0.1.0` twice is rejected, so cut `0.1.1`.

</Step>
<Step title="Use it">

The plugin is loadable as soon as the push is accepted. Declare a pipeline on it as on any core plugin:

```hcl
resource "marmot_pipeline" "internal_jobs" {
  name      = "internal-jobs"
  plugin_id = "internal-scheduler"

  config = jsonencode({
    endpoint = "https://scheduler.acme.internal"
  })

  cron_expression = "0 */4 * * *"
}
```

Credentials go through a [secret store](secret-stores.md) and the `secrets` map, exactly as for a core plugin.

</Step>
</Steps>

## Push from CI

A version pushed from a laptop is one nobody can reproduce. Give the release workflow a service account with `plugin.publisher`, which grants pushing and nothing else, and hand it the key in the same apply:

```hcl
resource "marmot_service_account" "plugin_release" {
  name        = "plugin-release"
  description = "Pushes internal plugins from the release workflow. Owned by the platform team."
}

resource "marmot_organization_iam_member" "plugin_release_publishes" {
  role   = "plugin.publisher"
  member = "serviceAccount:${marmot_service_account.plugin_release.id}"
}

resource "marmot_service_account_api_key" "plugin_release" {
  service_account_id = marmot_service_account.plugin_release.id
  name               = "release-workflow"
  expires_in_days    = 90
}

resource "github_actions_secret" "marmot_api_key" {
  repository      = "marmot-plugin-internal-scheduler"
  secret_name     = "MARMOT_API_KEY"
  plaintext_value = marmot_service_account_api_key.plugin_release.key
}
```

The workflow signs in with the key as the password and any username:

```bash
echo "$MARMOT_API_KEY" | oras login acme.marmotdata.cloud -u ci --password-stdin
```

## Versions

A pipeline can pin a plugin version or track the newest push. The pin is set in the pipeline editor in your instance. Leave pipelines unpinned while you iterate; pin them once people rely on them, so a routine push cannot change the code behind a production pipeline.

<CalloutCard
  title="Write your first plugin"
  description="The plugin SDK, the config spec, asset schemas and the local development loop."
  docId="Develop/creating-plugins"
  buttonText="Creating plugins"
  icon="mdi:puzzle-plus"
/>
