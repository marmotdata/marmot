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

## Tools

Besides discovery, the plugin can make a small set of typed changes, which a runner allowed to take tools runs on your behalf. Every tool is previewed with a server-side dry run first, and runs only if the object is still exactly as previewed.

| Tool | Changes | Undone by |
| --- | --- | --- |
| `scale` | A Deployment's or StatefulSet's replica count | `scale` back to the previous count |
| `rollout_restart` | A Deployment's, StatefulSet's or DaemonSet's pods, with a rolling update | Nothing needed: the workload returns to the same state |
| `rollout_undo` | A Deployment's pod template, restored from an earlier revision | `rollout_undo` to the revision it replaced (the `marmot-kubernetes-tools` chart refuses it; see its README) |
| `cordon`, `uncordon` | Whether a node accepts new pods | Each other |
| `suspend_cronjob`, `resume_cronjob` | Whether a CronJob starts new runs | Each other |

Each change is made with the field manager `marmot` and leaves `marmot.io/origin` (the run that made it) and `marmot.io/actor` (who asked) annotations on the object. No tool reads a Secret, reaches into a pod, or creates or deletes anything.

Objects that Flux or Argo CD manage are refused, since their changes belong in Git and the next reconcile would revert them. Limit tools further with:

```yaml
tools:
  max_replicas: 20
  namespaces:
    - "payments"
  selector: "marmot.io/tools=enabled"
  allow_gitops_managed: false
```

Without `namespaces`, tools may change every namespace except `kube-system`, `kube-public` and `kube-node-lease`. The [`marmot-kubernetes-tools` chart](https://github.com/marmotdata/marmot/tree/main/charts/marmot-kubernetes-tools) creates a service account with exactly the access these tools need, and an admission policy that refuses anything else from it.

### Reads

Three tools only read, so a task can look before it proposes a change. They need no plan and no approval, but the runner's policy must still name each one, and they keep to the same `namespaces` and `selector`. Objects Flux or Argo CD manage are readable.

| Read | Returns |
| --- | --- |
| `workload_status` | A Deployment's, StatefulSet's or DaemonSet's replica counts and conditions, and up to 20 of its pods, failing ones first, with phase, readiness, restarts, waiting reasons, last termination and how long each has been failing |
| `pod_logs` | Up to 200 of a container's last lines, or its previous instance's, with credentials redacted and each line capped at 2 KiB |
| `recent_events` | Up to 50 of a namespace's newest events, or one object's, with messages capped at 512 characters |

Logs and event messages are written by whatever runs in the cluster, so every read is marked untrusted. Redaction covers bearer tokens, `Authorization` headers, AWS access key ids and secret keys, private key blocks, `password`, `secret`, `token` and `api_key` values and anything shaped like a JWT. It is a safeguard, not a guarantee.

## Triggers

Triggers watch the cluster and report what goes wrong as small, typed events. They only read. Which triggers run, and every parameter, comes from the runner's own policy: Marmot never sends the plugin a trigger, a parameter or an expression, it only receives the events.

| Trigger | Fires when |
| --- | --- |
| `workload_unhealthy` | A workload or its pods have been `crash_looping`, `image_pull_failing`, `config_error`, `oom_killed`, `not_ready`, `unschedulable`, `replicas_unavailable`, `rollout_stuck` or `job_failed` for `for_minutes`. A pod's condition is reported against the workload that owns it |
| `node_unhealthy` | A node has been `not_ready`, under `memory_pressure`, `disk_pressure` or `pid_pressure`, `network_unavailable`, or cordoned (`unschedulable_for`) for `for_minutes` |
| `warning_events` | An object has gathered `min_count` Warning events of one listed reason within `window_minutes`. Events are best effort, so prefer the state-based triggers; this one is for what state does not show, such as failed mounts |
| `custom_condition` | A CEL expression you write over `object` has been true for `for_minutes`, such as `object.status.?availableReplicas.orValue(0) < object.spec.replicas` on Deployments. It must compile and return a bool when the watch starts, and an evaluation error counts as false |

Each event fires once per object and condition while it holds, and can fire again after it clears. Its facts are fixed: `kind`, `namespace`, `name`, `workload`, `condition`, `reason`, `pods_affected` or `count`, `restart_count` where relevant, and `since`. Free text, such as an event message, travels only in the untrusted `detail`.

Triggers watch the namespaces in `tools.namespaces`, or fewer. Nodes, and watching every namespace, need the chart's `triggers.clusterWide`.
