---
title: Let tasks act on a Kubernetes cluster
description: Run a runner in your cluster so tasks can read workload status and logs, start when something breaks and propose fixes people approve.
---

import { Steps, Step } from '@site/src/components/Steps';

# Let tasks act on a Kubernetes cluster

Run a runner in your cluster so [tasks](../Cloud/tasks.md) can look at what's wrong, start on their own when a workload breaks and propose a fix for people to approve. The runner connects out to Marmot, holds the cluster credentials and decides what it will do. [Runners](../Configure/serving-plugins.md) explains what it trusts.

## Before you start

- You can install a Helm chart in the cluster and create a namespace.
- The cluster can reach Marmot on port 443 and on the serve port, 8443.
- You have a Marmot service account with **Serve ingestion** (`ingestion:serve`) and its API key. See [Serve queries from your network](serve-queries.md#serve-the-source).
- Tasks are turned on for your instance.

## How it works

A chart gives the runner's service account only the access each action needs, and an admission policy refuses any other change from it. The runner's policy file then names the reads, actions and triggers it serves and how many people must approve each change.

## Set it up

<Steps>
<Step title="Install the actions chart">

The chart creates the runner's service account, a Role in each namespace you list and the admission policy. List the namespaces tasks may touch:

```bash
helm install marmot-actions marmotdata/marmot-kubernetes-actions \
  --namespace marmot --create-namespace \
  --set 'namespaces={payments,checkout}'
```

Scale, restart, the reads and namespaced triggers are on by default. Cordoning nodes, and triggers that watch nodes or every namespace, are off until you turn them on with `actions.cordon` and `triggers.clusterWide`. Every value is in the [runner reference](../Configure/serving-reference.md#kubernetes-actions-chart).

</Step>
<Step title="Write the pipeline">

The pipeline catalogs the cluster and sets the plugin's own limits on actions. Keep `actions.namespaces` the same as the chart's `namespaces`:

```yaml
name: prod-cluster
runs:
  - kubernetes:
      cluster_name: prod-cluster
      namespaces: [payments, checkout]
      actions:
        namespaces: [payments, checkout]
        max_replicas: 10
      queryable: true
```

The source takes the pipeline's name, so tasks see it as `prod-cluster`. Run in the cluster as the chart's service account, the plugin needs no host or token.

</Step>
<Step title="Write the runner's policy">

The policy names every read, action and trigger the runner serves. Anything it leaves out is refused, whatever Marmot asks:

```yaml
approvals:
  trust: marmot
  max_age: 15m
sources:
  prod-cluster:
    query: false
    origins: ["task_run:*"]
    actions:
      workload_status: {}
      pod_logs: {}
      recent_events: {}
      rollout_restart:
        approvals: 1
      scale:
        approvals: 2
        params:
          namespace: {values: [payments]}
          replicas: {min: 1, max: 5}
    triggers:
      workload_unhealthy:
        params:
          namespaces: [payments, checkout]
          conditions: [crash_looping, oom_killed]
          for_minutes: 10
        cooldown: 30m
```

This runner lets tasks read status, logs and events, restart a workload with 1 approval and scale one in `payments` between 1 and 5 replicas with 2 approvals. It tells Marmot when a workload has been crash looping or running out of memory for 10 minutes.

</Step>
<Step title="Start the runner">

Run it as a Deployment in the `marmot` namespace with the chart's service account, `marmot-actions`. Keep the API key in a Secret and the pipeline and policy in a ConfigMap:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: marmot-runner
  namespace: marmot
spec:
  replicas: 1
  selector:
    matchLabels: {app: marmot-runner}
  template:
    metadata:
      labels: {app: marmot-runner}
    spec:
      serviceAccountName: marmot-actions
      containers:
        - name: runner
          image: ghcr.io/marmotdata/marmot:<version>
          args:
            - plugin
            - -c=/etc/marmot/pipeline.yaml
            - --serve
            - --allow-actions
            - --policy=/etc/marmot/policy.yaml
            - --audit-log=/var/log/marmot/audit.jsonl
          env:
            - name: MARMOT_HOST
              value: https://<instance>.marmotdata.cloud
            - name: MARMOT_API_KEY
              valueFrom:
                secretKeyRef: {name: marmot-runner, key: api-key}
          volumeMounts:
            - {name: config, mountPath: /etc/marmot}
            - {name: audit, mountPath: /var/log/marmot}
      volumes:
        - name: config
          configMap: {name: marmot-runner}
        - name: audit
          persistentVolumeClaim: {claimName: marmot-runner-audit}
```

`--allow-actions` turns on reads, actions and triggers, and needs `--audit-log`. Keep the audit log on a volume that outlives the pod, because the runner uses it to refuse an approval used before a restart.

</Step>
<Step title="Optional: pin the plugin's actions">

When the runner starts, it logs the digest of the plugin's actions. Copy it into the policy so a plugin upgrade that changes any action, down to one bound, is refused until you review it:

```yaml
sources:
  prod-cluster:
    manifest: sha256:<digest>
```

</Step>
</Steps>

## Check that it worked

1. Open **Tasks** and select **New task**. **For your cluster** offers **Restart crash-looping deployments**.
2. Select **Start from scratch**, then choose **When something happens**. Under **Where from**, `prod-cluster` shows **Set up**.
3. Continue to **What may it do?**. It lists **Offered by the runner for prod-cluster**, with the reads and the actions your policy allows.

To check the runner's own record:

```bash
marmot ingest verify-audit-log /var/log/marmot/audit.jsonl
```

## Troubleshooting

| What you see | What to do |
| --- | --- |
| `--allow-actions needs --audit-log` | Add `--audit-log` with a writable path. |
| `action "<name>" is not in the plugin's manifest` or `has no parameter` | The policy names something the plugin doesn't offer. Check the spelling against the [plugin's actions](https://plugins.marmotdata.io). |
| `no approvers configured: the policy has no approvals section` | Add `approvals: {trust: marmot}` to the policy. |
| `the plugin's manifest is sha256:…, and the policy pins sha256:…` | The plugin's actions changed since you pinned them. Review what changed, then copy the new digest into `manifest`. |
| An action is listed under **Not allowed by the runner** | The policy doesn't name it. Add it and restart the runner. |
| An approved change fails with `forbidden` | The chart doesn't grant it, or the namespace isn't in the chart's `namespaces`. Upgrade the chart with the same namespaces as the pipeline. |
| An approved change fails with `denied request` from the admission policy | The change touches more than the action may, or the object lacks a label in `selector`. |
| A change is refused because the object is managed by Flux or Argo CD | Make the change in Git. |
| A change is marked stale | The object changed since the plan. Select **Plan again** on the change, then approve the new plan. |

## Next steps

- [Create a task](../Cloud/create-a-task.md): start a task when a workload keeps crashing.
- [Approve changes](../Cloud/approve-changes.md): review, approve and undo what tasks propose.
- [Runner reference](../Configure/serving-reference.md): every trigger, policy key and chart value.
