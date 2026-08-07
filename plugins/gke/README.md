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
