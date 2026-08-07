The EKS plugin discovers namespaces, services, deployments, stateful sets, cron jobs, and pods from Amazon EKS clusters. It is the [Kubernetes plugin](./Kubernetes)'s discovery engine with AWS IAM authentication, so the assets, lineage, and run history it produces are identical. See the Kubernetes plugin for details on what gets discovered and how resources are linked.

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
