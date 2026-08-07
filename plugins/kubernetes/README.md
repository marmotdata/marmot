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

- Amazon EKS: the [EKS plugin](./EKS)
- Google GKE: the [GKE plugin](./GKE)

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
