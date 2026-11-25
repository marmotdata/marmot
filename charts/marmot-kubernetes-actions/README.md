# marmot-kubernetes-actions

Gives a Marmot runner exactly the access the Kubernetes plugin's actions need in your cluster, and an admission policy that holds it to the changes those actions make.

Actions only ever run in your own runner (`marmot ingest --serve`), never on Marmot's servers. This chart is the cluster's side of that boundary: even a runner that misbehaves, or a token taken from it, can do no more than what is listed here.

## Install

```bash
helm install marmot-actions ./charts/marmot-kubernetes-actions \
  --namespace marmot --create-namespace \
  --set 'namespaces={payments,orders}'
```

Run the runner as the ServiceAccount this creates, or give it a token for that ServiceAccount. The admission policy recognises the runner by that ServiceAccount's username, so any other identity is not covered by it.

Keep `namespaces`, `maxReplicas` and `selector` in step with the plugin's `actions.namespaces`, `actions.max_replicas` and `actions.selector`. The plugin refuses first, with a clearer error; the cluster refuses as well.

## What is granted

One Role and RoleBinding in each namespace in `namespaces`, with only what the switched on verbs need:

| Verb | Resources | Access |
| --- | --- | --- |
| `scale` | Deployments, StatefulSets | get, list, watch, patch |
| | HorizontalPodAutoscalers | get, list, to warn when one would override a scale |
| `rollout_restart` | Deployments, StatefulSets, DaemonSets | get, list, watch, patch |
| | ReplicaSets | get, list, to find the revision to restore |
| `suspend_cronjob`, `resume_cronjob` | CronJobs | get, list, watch, patch |
| `workload_status`, `pod_logs`, `recent_events` (`actions.observe`) | Pods, Events (core and `events.k8s.io`) | get, list, watch |
| | `pods/log` | get |
| | Deployments, StatefulSets, DaemonSets, Jobs, CronJobs | get, list |
| Triggers (`triggers.enabled`) | Pods, Events, PersistentVolumeClaims, ReplicaSets, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, HorizontalPodAutoscalers | get, list, watch |

With `actions.cordon` on, one ClusterRole grants get, list, watch and patch on Nodes, since nodes belong to no namespace. List lets the plugin refuse to cordon the last schedulable node.

With `triggers.clusterWide` on, a second ClusterRole grants the triggers' get, list and watch across the whole cluster, and on Nodes. It is needed for `node_unhealthy`, a `custom_condition` on nodes, and any trigger watching every namespace rather than those in `namespaces`. It lets the runner read the status of every pod in the cluster, though still no Secret and no log, so it is off by default.

The reads and triggers write nothing, so the admission policy below does not concern them.

Nothing else is granted. In particular there is no access to Secrets, no create or delete on anything, no writes to pods, no `exec`, `attach` or `portforward`, no `log` unless `actions.observe` is on, no RBAC, and no `escalate`, `bind` or `impersonate`. The `scale` subresources are not granted either: the plugin writes the replica count on the workload itself, so the same write carries its audit annotations and the admission policy sees the whole object.

## What the admission policy refuses

A ValidatingAdmissionPolicy, bound to every namespace but matching only requests from the runner's ServiceAccount, so nothing else in the cluster is affected. It refuses, from the runner:

- creating or deleting anything, or writing through a subresource
- changing any label
- changing any annotation other than `marmot.io/origin` and `marmot.io/actor`, which record the run and the person behind each change
- changing a pod template in any way other than its `kubectl.kubernetes.io/restartedAt` annotation
- more than `maxReplicas` replicas
- changing a CronJob's schedule, time zone or job template
- changing a node's taints
- changing an object without every label in `selector`

If the policy cannot be evaluated, the change is refused.

### Rolling back

The chart does not grant `rollout_undo`, and its admission policy refuses every pod template change except the restart annotation. A rollback restores an earlier template, and admission cannot see a Deployment's history, so it cannot tell a genuine rollback from a new image or a privileged sidecar. In a test against a real API server, a rule that only kept the pod's identity, volumes and security context fixed still let the service account add a privileged container. Roll back through Git or by hand.

## Requirements

Kubernetes 1.30 or later, where ValidatingAdmissionPolicy is generally available.

## Values

| Value | Default | Meaning |
| --- | --- | --- |
| `serviceAccount.create` | `true` | Create the runner's ServiceAccount |
| `serviceAccount.name` | release name | Name of the ServiceAccount |
| `serviceAccount.annotations` | `{}` | Annotations, for example for cloud workload identity |
| `namespaces` | `[]` | Namespaces actions may change; required for any namespaced verb |
| `actions.scale` | `true` | Grant `scale` |
| `actions.rolloutRestart` | `true` | Grant `rollout_restart` |
| `actions.cronJobs` | `true` | Grant `suspend_cronjob` and `resume_cronjob` |
| `actions.cordon` | `false` | Grant `cordon` and `uncordon` |
| `actions.observe` | `true` | Grant the reads `workload_status`, `pod_logs` and `recent_events` |
| `triggers.enabled` | `true` | Grant what triggers watch, in each namespace in `namespaces` |
| `triggers.clusterWide` | `false` | Grant it across the cluster, and Nodes, through a ClusterRole |
| `maxReplicas` | `20` | Highest replica count admitted, from 1 to 1000 |
| `selector` | `{}` | Labels every changed object must carry, nodes included |
| `admissionPolicy.enabled` | `true` | Install the admission policy |
