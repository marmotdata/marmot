---
sidebar_position: 4
title: Kubernetes Operator
---

# Kubernetes Operator

Manage ingestion pipelines declaratively using the Marmot Operator.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

Instead of running `marmot ingest` from the CLI scripts or UI, the operator lets you define pipelines as Kubernetes resources. The cluster handles scheduling, retries and lifecycle for you. Pipeline config lives alongside your other manifests, so changes go through the same review and GitOps workflow as everything else.

The operator watches `Run` resources and reconciles them into Kubernetes Jobs or CronJobs. This allows you to run each job as a seperate pod on Kubernetes allowing for a more granular permisions model so you don't have to give Marmot acess to all your assets. It can also help with performance if you're ingesting a lot of assets regularly.

<TipBox variant="info" title="Prerequisites">
The operator is deployed alongside Marmot via the Helm chart. See the [Helm / Kubernetes](/docs/Deploy/Helm) guide to install Marmot first.
</TipBox>

## Enabling the Operator

Enable the operator in your Helm values:

```yaml
operator:
  enabled: true
```

```bash
helm upgrade marmot marmotdata/marmot -f values.yaml
```

## Creating a Run

A `Run` resource defines an ingestion pipeline. The `spec.runs` array uses the same format as the [CLI configuration file](/docs/Populating/CLI#configuration-file).

```yaml
apiVersion: runs.marmotdata.io/v1alpha1
kind: Run
metadata:
  name: my-pipeline
spec:
  schedule: "0 */6 * * *"
  runs:
    - postgresql:
        host: "db.example.com"
        port: 5432
        database: "production"
        user: "readonly"
```

The resource's `metadata.name` is used as the pipeline name for tracking ingestion state.

```bash
kubectl apply -f my-pipeline.yaml
```

## Pod Labels and Annotations

Use `podLabels` and `podAnnotations` to integrate ingestion pods with service meshes, observability tools or policy engines.

On AWS, this is particularly useful for providing credentials to plugins via [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html). Instead of storing AWS credentials in your pipeline config, annotate the pod so it automatically receives IAM permissions:

```yaml
spec:
  podAnnotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/marmot-s3-reader"
  podLabels:
    team: data-engineering
  runs:
    - s3:
        bucket: "my-data-lake"
        region: "eu-west-1"
```

## Manual Triggers

Trigger a scheduled pipeline outside its cron window by annotating the Run:

```bash
kubectl annotate run my-pipeline runs.marmotdata.io/trigger=true
```

This creates a temporary Job that runs immediately and cleans up after 60 seconds.

## Teardown on Delete

By default, deleting a Run resource runs `marmot ingest --destroy` to remove all assets that pipeline previously discovered from Marmot. Set `teardownOnDelete: false` if you want to keep existing assets after removing the Run.

## Reference

### Run Spec

| Field                        | Type                                                                                                                      | Default  | Description                                                     |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------- | -------- | --------------------------------------------------------------- |
| `runs`                       | array                                                                                                                     | required | Source configurations, same format as CLI YAML                  |
| `schedule`                   | string                                                                                                                    |          | Cron expression. When set, creates a CronJob instead of a Job   |
| `suspend`                    | boolean                                                                                                                   | `false`  | Pause scheduled executions. Only applies when `schedule` is set |
| `concurrencyPolicy`          | `Allow` / `Forbid` / `Replace`                                                                                            | `Forbid` | How to handle concurrent Job executions                         |
| `backoffLimit`               | int                                                                                                                       | `3`      | Retries before marking a Job as failed                          |
| `activeDeadlineSeconds`      | int                                                                                                                       |          | Maximum duration (seconds) a Job may run                        |
| `successfulJobsHistoryLimit` | int                                                                                                                       | `3`      | Successful CronJob runs to retain                               |
| `failedJobsHistoryLimit`     | int                                                                                                                       | `1`      | Failed CronJob runs to retain                                   |
| `resources`                  | [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core) |          | CPU/memory requests and limits for the ingestion container      |
| `podLabels`                  | map                                                                                                                       |          | Additional labels applied to the pod template                   |
| `podAnnotations`             | map                                                                                                                       |          | Additional annotations applied to the pod template              |
| `teardownOnDelete`           | boolean                                                                                                                   | `true`   | Run `marmot ingest --destroy` when the Run is deleted           |

### Operator Helm Values

| Key                       | Default                | Description                               |
| ------------------------- | ---------------------- | ----------------------------------------- |
| `operator.enabled`        | `false`                | Enable the operator Deployment and CRD    |
| `operator.replicas`       | `1`                    | Number of operator replicas               |
| `operator.leaderElect`    | `true`                 | Enable leader election for HA             |
| `operator.watchNamespace` | `""` (all)             | Restrict to a single namespace            |
| `operator.marmot.url`     | auto-detected          | Marmot API URL passed to Job pods         |
| `operator.resources`      | 100m/128Mi, 500m/256Mi | Operator pod resource requests and limits |

## Next Steps

<DocCardGrid>
  <DocCard
    title="Browse Plugins"
    description="See all available data source plugins and their configuration"
    href="/docs/Plugins"
    icon="mdi:puzzle"
  />
  <DocCard
    title="CLI Ingestion"
    description="Learn the pipeline config format used by Run resources"
    href="/docs/Populating/CLI"
    icon="mdi:console"
  />
</DocCardGrid>
