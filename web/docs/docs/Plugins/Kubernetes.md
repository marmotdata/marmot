---
title: Kubernetes
description: Discovers namespaces, services, deployments, stateful sets, cron jobs, and pods from Kubernetes clusters.
status: experimental
---

# Kubernetes

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


The Kubernetes plugin discovers namespaces, services, deployments, stateful sets, cron jobs, and pods from Kubernetes clusters. Each resource kind can be toggled on or off, and discovery can be scoped to specific namespaces or a label selector.

Discovered resources are linked together: namespaces contain their resources, services link to the deployments and stateful sets they expose (matched by selector), and workloads link to their pods (matched by owner references). Cron jobs come with run history built from their recent job runs, so the catalog shows whether the nightly pipeline actually succeeded. When `cluster_name` is set, a Cluster asset is created as the root of the tree.

Pods are not discovered by default because they are short-lived and can flood the catalog; enable `discover_pods` when pod-level visibility is worth the churn. One-off Jobs are never cataloged for the same reason; only jobs owned by a cron job are used, as run history.

## Prerequisites

The plugin needs read access to the resources it discovers. When running inside a cluster, bind a role like this to the service account Marmot runs as:

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

:::tip[Managed clusters]
This plugin is for self-managed and on-prem clusters. For managed clusters that authenticate with cloud IAM, use the dedicated plugins, which reuse this plugin's discovery engine:

- Amazon EKS: the [EKS plugin](/docs/Plugins/EKS)
- Google GKE: the [GKE plugin](/docs/Plugins/GKE)

:::

:::tip[Authentication]
The plugin supports three authentication methods:

- In-cluster: when Marmot runs inside Kubernetes and no connection settings are provided, the pod's service account is used automatically. The projected token is rotated automatically, so there is nothing to refresh.
- Kubeconfig: `$KUBECONFIG` or `~/.kube/config` is used when Marmot runs somewhere kubectl already works. Set `kubeconfig_path` and `context` to pick a specific file and context.
- Direct token: set `host`, `token`, and `ca_certificate` to connect to any cluster with a service account token.

:::

### Connecting with a service account token

Create a service account bound to the read-only role above and mint a token for it:

```bash
kubectl create serviceaccount marmot-discovery
kubectl create clusterrolebinding marmot-discovery \
  --clusterrole=marmot-discovery --serviceaccount=default:marmot-discovery
kubectl create token marmot-discovery --duration=48h
```

:::warning[Tokens expire]
`kubectl create token` mints a time-bounded token, and the API server caps the lifetime (often 48h) regardless of the `--duration` you request, so a scheduled ingest will start failing once it expires. For unattended discovery, prefer in-cluster auth (its token is rotated automatically), or rotate the token on a schedule. A long-lived token can be created with a [`kubernetes.io/service-account-token` Secret](https://kubernetes.io/docs/concepts/configuration/secret/#service-account-token-secrets), but that is discouraged upstream and disabled on some clusters.
:::

Then give the plugin the cluster endpoint, the token, and the cluster's CA certificate. The connection fields go in the same config as the discovery options, not a separate file:

```yaml
host: "https://mycluster.example.com:6443"
token: "${K8S_SA_TOKEN}"
ca_certificate: |
  -----BEGIN CERTIFICATE-----
  ...
  -----END CERTIFICATE-----
cluster_name: "prod"
namespaces:
  - "payments"
  - "orders"
discover_pods: false
tags:
  - "kubernetes"
  - "${labels.team}"
```

## Example Configuration

When Marmot runs inside the cluster or has a working kubeconfig, no connection fields are needed; leave out `host`/`token` and the plugin uses the in-cluster service account or your kubeconfig. This example lists every discovery option with its default:

```yaml

cluster_name: "prod"
namespaces:
  - "payments"
  - "orders"
discover_namespaces: true
discover_services: true
discover_deployments: true
discover_statefulsets: true
discover_cronjobs: true
discover_pods: false
labels_to_metadata: true
annotations_to_metadata: false
tags:
  - "kubernetes"
  - "${labels.team}"

```

Set `namespaces` to `["*"]` (or leave it empty) to discover all namespaces except the ones in `exclude_namespaces`. Tags interpolate resource labels, so `${labels.team}` tags every asset with the value of its `team` label. Set `cluster_name` when cataloging more than one cluster; it prefixes asset names (`prod/payments/api`) so the same namespace in two clusters stays distinct, and it creates a Cluster asset that anchors the lineage tree.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| annotations_to_metadata | bool | false | Include resource annotations in asset metadata |
| ca_certificate | string | false | PEM-encoded CA certificate of the API server |
| cluster_name | string | false | Cluster name to prefix asset names with |
| context | string | false | Kubeconfig context. Defaults to the current context |
| host | string | false | API server URL for direct token authentication |
| discover_cronjobs | bool | false | Discover cron jobs, with their recent job runs as run history |
| discover_deployments | bool | false | Discover deployments |
| discover_namespaces | bool | false | Discover namespaces |
| discover_pods | bool | false | Discover pods. Off by default because pods are short-lived and can flood the catalog |
| discover_services | bool | false | Discover services |
| discover_statefulsets | bool | false | Discover stateful sets |
| exclude_namespaces | []string | false | Namespaces to skip when discovering all namespaces |
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| kubeconfig_path | string | false | Kubeconfig path. Defaults to in-cluster, then $KUBECONFIG |
| label_selector | string | false | Only discover namespaced resources matching this label selector (e.g. team=data) |
| labels_to_metadata | bool | false | Include resource labels in asset metadata |
| namespaces | []string | false | Namespaces to discover. Empty or ["*"] means all namespaces |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| token | string | false | Bearer token, typically a service account token |

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| annotations | map[string]string | Resource annotations |
| available_replicas | int32 | Number of available replicas |
| cluster | string | Configured cluster name |
| cluster_ip | string | Cluster IP address (None for headless services) |
| concurrency_policy | string | Cron job concurrency policy (Allow, Forbid, Replace) |
| container_count | int | Number of containers in the pod template |
| created_at | string | Resource creation timestamp |
| external_name | string | External DNS name for ExternalName services |
| headless_service | string | Headless service governing the stateful set |
| images | string | Container images (comma-separated) |
| kubernetes_version | string | Kubernetes server version |
| labels | map[string]string | Resource labels |
| last_schedule_time | string | When the cron job last fired |
| last_successful_time | string | When the cron job last completed successfully |
| load_balancer_hosts | string | Load balancer ingress hostnames and IPs |
| namespace | string | Namespace name |
| node | string | Node the pod is scheduled on |
| owner_kind | string | Kind of the controlling owner (ReplicaSet, StatefulSet, DaemonSet, Job) |
| owner_name | string | Name of the controlling owner |
| paused | bool | Whether rollouts are paused |
| phase | string | Lifecycle phase (Active, Running, Pending, Failed) |
| platform | string | Server platform (e.g. linux/amd64) |
| ports | string | Exposed ports (name:port/protocol, comma-separated) |
| qos_class | string | Quality of service class (Guaranteed, Burstable, BestEffort) |
| ready_replicas | int32 | Number of ready replicas |
| replicas | int32 | Desired replica count |
| restart_count | int32 | Total container restarts |
| schedule | string | Cron schedule expression |
| selector | string | Pod selector labels (key=value, comma-separated) |
| service_account | string | Service account the resource runs as |
| service_type | string | Service type (ClusterIP, NodePort, LoadBalancer, ExternalName) |
| strategy | string | Rollout/update strategy (RollingUpdate, Recreate, OnDelete) |
| suspended | bool | Whether the cron job is suspended |
| timezone | string | Time zone the cron schedule is evaluated in |
| updated_replicas | int32 | Number of replicas updated to the latest pod template |
| volume_claims | string | Volume claim templates (name:size/storageClass, comma-separated) |
