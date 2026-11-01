---
sidebar_position: 10
title: Plugin Updates
---

# Plugin Updates

Plugins are released separately from Marmot, so a plugin can move to a newer version without a Marmot upgrade. This is how security fixes in plugins reach your install quickly.

Each Marmot release pins a version of every core plugin, and a fresh install runs those versions. From there, Marmot checks a signed plugin index in the background. The index lists every published plugin version, the Marmot releases each version supports and any security advisories against them. Checking the index never slows startup, and a Marmot that can't reach it keeps running the versions it has.

## How Versions Are Chosen

For each core plugin, Marmot runs the first of these that applies:

1. **A local plugin** in `~/.marmot/plugins` (or `MARMOT_PLUGINS_DIR`). It overrides everything below, which is useful for development.
2. **A version you or the auto-update policy chose**, stored in the database so every replica runs the same version. The exception is a Marmot upgrade that pins a newer version than the stored one: the newer version wins unless you pinned the plugin.
3. **The version this Marmot release pins.**

When a plugin's version changes, every replica switches to it without a restart. Pipeline runs and queries already in progress finish on the old version, and anything that starts afterwards uses the new one. Before a pipeline run starts, Marmot also confirms the plugin is at its chosen version, so a replica that missed a change never runs the old one.

## Managing Versions

Open **Settings → Plugins** to see each plugin's version, where that version came from and whether an update or security advisory applies to it. Users who can manage ingestion can:

- **Update** a plugin to the latest version, or to any version that supports this Marmot release. Versions that need a newer Marmot are shown but can't be selected.
- **Pin** a plugin so that auto-update and Marmot upgrades leave its version alone.
- **Roll back** to the version the plugin ran before its last change. Rolling back also pins the plugin, so auto-update doesn't immediately reapply the version you just left.
- **Reset** a plugin to the version this Marmot release pins.

When a plugin you run has a security advisory against it, users who can manage ingestion see a banner across Marmot until the plugin is updated. Ordinary updates only show on the Plugins page.

The CLI offers the same actions:

```bash
marmot plugins list              # every core plugin, its version and status
marmot plugins outdated          # only plugins with an update or advisory
marmot plugins update postgresql # to the latest version; or postgresql@0.3.1
marmot plugins pin postgresql
marmot plugins rollback postgresql
marmot plugins reset postgresql
marmot plugins check             # check the index now
```

## Auto-Update

The `plugins.updates.auto_update` setting decides what Marmot applies without anyone acting:

| Policy     | Behaviour                                                                                                                           |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `security` | The default. A plugin with an advisory against its version moves to the smallest version that fixes it and supports this Marmot. |
| `patch`    | As `security`, and also applies new patch releases within the plugin's current minor version.                                    |
| `off`      | Nothing changes automatically. Advisories still show in the UI.                                                                     |

Pinned plugins are never auto-updated. Every change records who made it (`auto-update` for automatic changes) and the version it replaced.

## Configuration

| Key                               | Description                                                                         | Default                    |
| --------------------------------- | ----------------------------------------------------------------------------------- | -------------------------- |
| `plugins.updates.enabled`         | Check the plugin index for new versions and advisories                              | `true`                     |
| `plugins.updates.auto_update`     | `off`, `security` or `patch`                                                        | `security`                 |
| `plugins.updates.check_interval`  | Seconds between index checks. A single replica performs each check.                 | `43200`                    |
| `plugins.updates.index`           | OCI reference of the plugin index, or an `https://` URL such as `https://plugins.marmotdata.io/index.json` | `<plugins.registry>/index:latest` |

With `plugins.updates.enabled: false`, Marmot makes no index requests. Plugins run their release versions, or whatever versions were chosen before checks were turned off.

## Trust and Air-Gapped Installs

The index is signed, and Marmot only accepts it when the signature matches a key built into the Marmot binary. Marmot then installs a plugin version only by the digest the index lists for it. As a result, neither a compromised registry nor a mirror can change which code runs. Marmot also refuses an index older than the newest one it has seen, and an expired index. Together these checks stop a captured index from being replayed to hide newer advisories.

The index is built from the [plugin registry](https://plugins.marmotdata.io), which shows the same versions, supported Marmot releases and advisories. It's published as an OCI artifact next to the plugins, so a registry mirror carries it as well, and at `https://plugins.marmotdata.io/index.json` for anyone who would rather fetch it over HTTPS. To run without internet access, mirror `ghcr.io/marmotdata/plugins` (including the `index` repository) into your registry and point `plugins.registry` at the mirror. Mirroring doesn't break the signature, which is checked against the key in the binary, not against the registry the index came from.

## For Plugin Authors

A plugin declares the Marmot releases it supports with `MarmotVersion` in its metadata. Every core plugin sets one, and CI refuses to test or release a core plugin without it:

```go
pluginsdk.Meta{
    ID:            "postgresql",
    MarmotVersion: ">=0.12.0",
    // ...
}
```

Prefer a lower bound only. An incompatible change to how Marmot talks to plugins bumps the plugin protocol version, which the SDK reports automatically, so an upper bound isn't needed. Marmot refuses to load a plugin that excludes it, and the plugin index never offers that version to Marmot releases outside the range.
