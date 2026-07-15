---
title: GKE
description: Discovers namespaces, services, workloads, and cron jobs from Google GKE clusters.
status: experimental
---

# Google GKE

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Run History</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The GKE plugin discovers namespaces, services, deployments, stateful sets, cron jobs, and pods from Google Kubernetes Engine clusters. It is the [Kubernetes plugin](./Kubernetes)'s discovery engine with Google Cloud authentication, so the assets, lineage, and run history it produces are identical. See the Kubernetes plugin for details on what gets discovered and how resources are linked.

Authentication uses Google Cloud IAM: on each run the plugin mints a short-lived OAuth token from the Google credentials of wherever Marmot runs. There is no static token to store or rotate. This is the clean way to read a GKE cluster from a GCE instance, Cloud Run, or another Google Cloud workload.

## Prerequisites

The identity Marmot runs as needs read access to the cluster, granted two ways:

First, a Google Cloud IAM role that allows connecting to the cluster (for example `roles/container.viewer`), so Google authorizes the token.

Second, a read-only Kubernetes RBAC role bound to that identity:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: marmot-discovery
rules:
  - apiGroups: [""]
    resources: ["namespaces", "services", "pods"]
    verbs: ["get", "list"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "replicasets"]
    verbs: ["get", "list"]
  - apiGroups: ["batch"]
    resources: ["cronjobs", "jobs"]
    verbs: ["get", "list"]
```

:::tip[Google credentials]
Credentials resolve from Application Default Credentials: Workload Identity, a Cloud Run or GCE service account, or `GOOGLE_APPLICATION_CREDENTIALS`. When Marmot runs outside Google Cloud, set `credentials.credentials_json` (or `credentials.credentials_file`) to a service account key.
:::

## Connecting to a cluster

Name the cluster and the plugin resolves its endpoint and CA certificate from the GKE management API. Set `project_id`, `location`, and `cluster`. This needs the `container.clusters.get` permission (included in `roles/container.viewer`).

```yaml
project_id: "my-project"
location: "us-central1"
cluster: "autopilot-cluster-1"
```

## Example Configuration

```yaml

project_id: "my-project"
location: "us-central1"
cluster: "autopilot-cluster-1"
namespaces:
  - "payments"
  - "orders"
discover_pods: false
tags:
  - "kubernetes"
  - "${labels.team}"

```

The discovery options (`namespaces`, `discover_*`, `cluster_name`, `tags`, and so on) are the same as the [Kubernetes plugin](./Kubernetes); see there for what each one does. The cluster name is used as the asset name prefix unless you set `cluster_name`.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| annotations_to_metadata | bool | false | Include resource annotations in asset metadata |
| cluster | string | true | GKE cluster name |
| cluster_name | string | false | Cluster name to prefix asset names with |
| credentials | GCPCredentials | false | GCP credentials configuration |
| discover_cronjobs | bool | false | Discover cron jobs, with their recent job runs as run history |
| discover_deployments | bool | false | Discover deployments |
| discover_namespaces | bool | false | Discover namespaces |
| discover_pods | bool | false | Discover pods. Off by default because pods are short-lived and can flood the catalog |
| discover_services | bool | false | Discover services |
| discover_statefulsets | bool | false | Discover stateful sets |
| exclude_namespaces | []string | false | Namespaces to skip when discovering all namespaces |
| label_selector | string | false | Only discover namespaced resources matching this label selector (e.g. team=data) |
| labels_to_metadata | bool | false | Include resource labels in asset metadata |
| location | string | true | Cluster region or zone, for example us-central1 |
| namespaces | []string | false | Namespaces to discover. Empty or ["*"] means all namespaces |
| project_id | string | true | GCP project ID |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The metadata fields are the same as the [Kubernetes plugin](./Kubernetes#available-metadata). Every asset also carries `cloud` (set to `GKE`), `gcp_project`, and `gcp_location`, so you can tell where a cluster lives without following lineage.
