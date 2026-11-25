---
title: Runner reference
description: Server settings, command flags, policy file keys, Kubernetes triggers and chart values for runners.
---

# Runner reference

Every setting for [runners](serving-plugins.md): the server's serving config, the runner's flags, its policy file, the Kubernetes plugin's triggers and the tools chart.

## Server configuration

Self-hosted servers only. Marmot Cloud sets these for you.

| Key | Description | Default |
| --- | --- | --- |
| `query.serve.enabled` | Accept runners | `false` |
| `query.serve.listen_address` | Address the serve listener binds | `:8443` |
| `query.serve.public_address` | `host:port` runners dial, when it isn't the Marmot host on the listen port | |
| `query.serve.replica_address` | Host or `host:port` other replicas forward queries to. Only with more than 1 replica | |
| `query.serve.ca_key_uri` | KMS key for the serving CA | |
| `query.serve.signing_key_uri` | KMS key that signs every request | |
| `query.serve.approval_key_uri` | KMS key that signs approvals, and nothing else | |

Serving needs the server encryption key, and Marmot won't start with serving on and no key. Name all 3 KMS keys or none. Each is an ECDSA P-256 key:

| KMS | URI |
| --- | --- |
| AWS KMS | `awskms:///<key-arn>` |
| Google Cloud KMS | `gcpkms://projects/<project>/locations/<location>/keyRings/<ring>/cryptoKeys/<key>/cryptoKeyVersions/<version>` |
| Azure Key Vault | `azurekms://<vault>.vault.azure.net/<key>` |
| HashiCorp Vault transit | `hashivault://<key>` |

## Runner flags

Flags of `marmot plugin`, also called `marmot ingest`:

| Flag | Description |
| --- | --- |
| `--serve` | After ingesting, stay running and serve the run marked `queryable: true` |
| `--policy <path>` | A YAML policy limiting what the runner serves. Once given, sources it doesn't name are refused |
| `--audit-log <path>` | Append a hash-chained JSON line for every query, read, plan, tool and trigger event |
| `--allow-tools` | Also serve the reads, tools and triggers the policy names. Needs `--audit-log` |
| `--ca-fingerprint sha256:<hex>` | A CA fingerprint Marmot's first enrolment must include. Repeatable |
| `--ca-file <path>` | A PEM CA to trust for the Marmot API, as well as the system's. Also `MARMOT_CA_FILE` |

The runner reads `MARMOT_HOST`, which must be HTTPS, and `MARMOT_API_KEY`.

## Commands

| Command | Description |
| --- | --- |
| `marmot admin serve-keys` | List the serving key generations, their fingerprints and when they activate and retire |
| `marmot admin rotate-serve-keys` | Rotate the serving keys gracefully. `--immediate` switches at once and closes every connection |
| `marmot ingest verify-audit-log <path>` | Check a runner's audit log for edited or removed lines |

`rotate-serve-keys` takes `--ca-key-uri`, `--signing-key-uri` and `--approval-key-uri` to move the keys into a KMS or change them. Name all 3 or none.

## Policy file

Unknown keys are an error, so a misspelt key can't loosen the policy. The runner checks the policy against the plugin's manifest when it starts.

### Top level

| Key | Description |
| --- | --- |
| `approvals.trust` | How the runner trusts approvals. `marmot` is the only value. Required when any write needs approval |
| `approvals.max_age` | The longest an approval stays valid, at most `1h` |
| `approvals.key_fingerprint` | Accept only the approval key with this fingerprint, from `marmot admin serve-keys` |
| `sources.<name>` | A source the runner serves, by its pipeline name |

### Sources

| Key | Description | Default |
| --- | --- | --- |
| `query` | Whether the source answers queries. `false` refuses them | `true` |
| `max_rows` | A ceiling on rows. A query signed with a higher limit is cut to it | No ceiling |
| `actors` | Patterns the signed actor must match, such as `user:alice` or `service_account:*` | Any actor |
| `origins` | Patterns the signed origin must match, such as `task_run:*`. Tools only ever come from task runs, whatever this says | Any origin |
| `manifest` | `sha256:<hex>` pin of the plugin's tools. Any change to them refuses every tool until updated | No pin |
| `tools.<name>` | A read or tool the runner serves. One the policy doesn't name is refused | |
| `triggers.<name>` | A trigger the runner runs | |

In patterns, `*` matches any run of characters.

### Tools

| Key | Description |
| --- | --- |
| `actors`, `origins` | Allow-lists checked as well as the source's |
| `params.<param>.min`, `.max` | Bounds for a whole-number parameter |
| `params.<param>.values` | The only values allowed. A constrained parameter must be given |
| `approvals` | Distinct people who must approve: 1 when unset, at most 2. `0` only for a risk 1, reversible tool, written out |
| `approvers` | Only these people count, by Marmot user id or user name |

Reads take no `approvals`.

### Triggers

| Key | Description | Default |
| --- | --- | --- |
| `params` | The trigger's parameters, checked against the plugin's manifest | |
| `cooldown` | How long repeats for the same object are dropped, from `1m` to `24h` | `30m` |

Each source sends at most 30 trigger events a minute.

## Kubernetes triggers

Every trigger only reads and is scoped by `namespaces` and `selector`. Nodes, and watching every namespace, need the chart's `triggers.clusterWide`.

| Trigger | Fires when | Parameters |
| --- | --- | --- |
| `workload_unhealthy` | A workload or its pods hold a condition for `for_minutes` | `kinds`, `namespaces`, `selector`, `conditions`, `for_minutes` |
| `node_unhealthy` | A node holds a condition for `for_minutes` | `selector`, `conditions`, `for_minutes` |
| `warning_events` | An object gathers `min_count` Warning events of one listed reason within `window_minutes` | `namespaces`, `reasons`, `min_count`, `window_minutes` |
| `custom_condition` | A CEL expression `when` over `object` is true for `for_minutes` | `kind`, `namespaces`, `selector`, `name`, `when`, `for_minutes` |

`workload_unhealthy` conditions are `crash_looping`, `image_pull_failing`, `config_error`, `oom_killed`, `not_ready`, `unschedulable`, `replicas_unavailable`, `rollout_stuck` and `job_failed`. `node_unhealthy` conditions are `not_ready`, `memory_pressure`, `disk_pressure`, `pid_pressure`, `network_unavailable` and `unschedulable_for`.

A `custom_condition` must compile and return a bool when the runner starts. An evaluation error counts as false. For example, a deployment with fewer replicas available than it wants:

```yaml
custom_condition:
  params:
    kind: deployment
    namespaces: [payments]
    name: below_desired
    when: object.status.?availableReplicas.orValue(0) < object.spec.replicas
    for_minutes: 5
  cooldown: 2h
```

## Kubernetes tools chart

Values of `marmotdata/marmot-kubernetes-tools`:

| Value | Description | Default |
| --- | --- | --- |
| `namespaces` | Namespaces tools may change. Each gets its own Role. Match the plugin's `tools.namespaces` | `[]` |
| `tools.scale` | Grant scaling Deployments and StatefulSets | `true` |
| `tools.rolloutRestart` | Grant restarting Deployments, StatefulSets and DaemonSets | `true` |
| `tools.cronJobs` | Grant suspending and resuming CronJobs | `true` |
| `tools.cordon` | Grant cordoning and uncordoning nodes, through a ClusterRole | `false` |
| `tools.observe` | Grant the reads: pods, their logs and events | `true` |
| `triggers.enabled` | Grant watching workloads and events in `namespaces` | `true` |
| `triggers.clusterWide` | Grant watching every namespace and nodes. Reads every pod's status, never a Secret or log | `false` |
| `maxReplicas` | Most replicas the admission policy admits. At or above the plugin's `tools.max_replicas` | `20` |
| `selector` | Labels every changed object must carry. Match the plugin's `tools.selector` | `{}` |
| `admissionPolicy.enabled` | Install the admission policy | `true` |
| `serviceAccount.name` | The runner's service account | The release name |

The chart grants no access to Secrets, no create or delete, no `exec` and nothing on RBAC. Its admission policy refuses, from the runner, any change beyond the fields a tool touches, and refuses every change if it can't be evaluated. It doesn't grant rolling back, because admission can't tell a rollback from any other change to a pod template.
