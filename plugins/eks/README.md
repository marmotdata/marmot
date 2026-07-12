---
title: EKS
description: Discovers namespaces, services, workloads, and cron jobs from Amazon EKS clusters.
status: experimental
---

# Amazon EKS

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


The EKS plugin discovers namespaces, services, deployments, stateful sets, cron jobs, and pods from Amazon EKS clusters. It is the [Kubernetes plugin](/docs/Plugins/Kubernetes)'s discovery engine with AWS IAM authentication, so the assets, lineage, and run history it produces are identical. See the Kubernetes plugin for details on what gets discovered and how resources are linked.

Authentication uses AWS IAM: on each run the plugin mints a short-lived token from the AWS credentials of wherever Marmot runs. There is no static token to store or rotate. This is the clean way to read an EKS cluster from an EC2 instance or another AWS workload.

## Prerequisites

Two grants are needed on the AWS side, plus the read-only Kubernetes RBAC role.

First, the IAM identity that Marmot runs as needs an [EKS access entry](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html) on the cluster (or a mapping in the older `aws-auth` ConfigMap).

Second, that access entry must map to a Kubernetes group bound to a read-only role:

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

:::tip[AWS credentials]
Credentials resolve from the standard AWS chain: IRSA, EKS Pod Identity, an EC2 instance profile, or static keys. Set `credentials.role` to assume a role, or `credentials.region` to pin the region. When Marmot runs outside AWS, set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` in its environment and the chain picks them up.
:::

## Connecting to a cluster

The plugin looks up the cluster's endpoint and CA certificate from the EKS API, so you only give it the cluster name and region. This needs the `eks:DescribeCluster` permission.

```yaml
eks_cluster_name: "prod"
credentials:
  region: "eu-west-1"
```

## Example Configuration

```yaml

eks_cluster_name: "prod"
credentials:
  region: "eu-west-1"
namespaces:
  - "payments"
  - "orders"
discover_pods: false
tags:
  - "kubernetes"
  - "${labels.team}"

```

The discovery options (`namespaces`, `discover_*`, `cluster_name`, `tags`, and so on) are the same as the [Kubernetes plugin](/docs/Plugins/Kubernetes); see there for what each one does. The cluster name is used as the asset name prefix unless you set `cluster_name`.

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| annotations_to_metadata | bool | false | Include resource annotations in asset metadata |
| cluster_name | string | false | Cluster name to prefix asset names with |
| credentials | AWSCredentials | false | AWS credentials configuration |
| discover_cronjobs | bool | false | Discover cron jobs, with their recent job runs as run history |
| discover_deployments | bool | false | Discover deployments |
| discover_namespaces | bool | false | Discover namespaces |
| discover_pods | bool | false | Discover pods. Off by default because pods are short-lived and can flood the catalog |
| discover_services | bool | false | Discover services |
| discover_statefulsets | bool | false | Discover stateful sets |
| eks_cluster_name | string | true | EKS cluster name |
| exclude_namespaces | []string | false | Namespaces to skip when discovering all namespaces |
| label_selector | string | false | Only discover namespaced resources matching this label selector (e.g. team=data) |
| labels_to_metadata | bool | false | Include resource labels in asset metadata |
| namespaces | []string | false | Namespaces to discover. Empty or ["*"] means all namespaces |
| tags | TagsConfig | false | Tags to apply to discovered assets |

## Available Metadata

The metadata fields are the same as the [Kubernetes plugin](/docs/Plugins/Kubernetes#available-metadata). Every asset also carries `cloud` (set to `EKS`), `aws_region`, and `aws_account_id`, so you can tell where a cluster lives without following lineage. The cluster asset additionally carries `cluster_arn`, its canonical AWS identifier.
