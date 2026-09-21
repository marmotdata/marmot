---
sidebar_position: 10
title: Security and isolation
description: How Marmot Cloud instances are kept apart, what an instance is allowed to reach, and how your data is protected.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { TipBox } from '@site/src/components/Steps';

# Security and isolation

Marmot Cloud runs every customer instance on shared infrastructure, so the boundaries between instances are the most important property of the platform. This page says where they are.

## Data stored

Metadata: asset names, schemas, ownership, descriptions, tags, lineage, glossary terms and statistics. Plugins read structure, never rows, messages or payloads.

Two exceptions, both under your control. Table preview, off by default, queries the source on demand and returns sample rows for display without storing them; reading one needs `asset.dataViewer`. Credentials written directly into a pipeline's `config` are stored encrypted on your instance, which is why [secret stores](secret-stores.md) and [keyless pipelines](pipelines.md#how-keyless-pipelines-work) are the documented paths: neither leaves anything to store.

## Isolation

Each instance gets its own Kubernetes namespace, created with it and deleted with it, under the restricted Pod Security Standard: nothing runs as root or gains capabilities. A quota forbids `LoadBalancer` and `NodePort` services, so the shared gateway is the only way in.

Network policy does the rest. Inbound is allowed only from the gateway, the database operator and the metrics collector, each on its own port. Outbound is denied by default and opened only by the egress rules below. Your Postgres cluster is unreachable from other instances, from the internet and from the gateway itself.

## Egress

Pipelines read your warehouses and brokers, and users authenticate against your identity provider, so an instance needs the outside world. It must not reach other instances, the cluster nodes, the Kubernetes API or the cloud metadata service.

**Internet**, the default, allows any public address and blocks private ranges and the metadata endpoint.

**Allowlist** allows only the destinations you name. [Tell support](mailto:support@marmotdata.io) what your instance may talk to, as CIDRs or domain names with ports:

```yaml
egress:
  mode: Allowlist
  allow:
    - cidr: 203.0.113.0/24
      ports: [5432]
    - fqdn: "*.snowflakecomputing.com"
      ports: [443]
```

A wildcard matches one label. The same list works additively in Internet mode, which is how you reach a peered private network without giving up the public default.

## Limits

Request limits at the gateway cover requests per second, burst, in-flight requests and body size, for all clients of an instance together. Egress bandwidth is capped per pod by [plan and size](plans.md#sizes); traffic above the cap is queued, not dropped. Inbound traffic from your own systems is not throttled.

## Encryption and backups

Every instance is served over TLS with an automatically renewed certificate. Traffic between gateway, server and Postgres stays inside the cluster network.

Storage is encrypted at rest. Each instance holds its own encryption key, created with it and destroyed with it, which protects the credentials Marmot does store.

Postgres is backed up nightly with continuous write-ahead log archiving, retained for thirty days. Point-in-time recovery is available from Scale; Enterprise can set a different retention. Restores are [a support request](mailto:support@marmotdata.io) naming the instance and the timestamp.

<TipBox variant="warning" title="Deleting an instance deletes its backups">
No grace period, no undelete. Export first with the [API](/docs/api-reference) or [CLI](/docs/cli).
</TipBox>

## Access and audit

Sign-in is [your identity provider](single-sign-on.md) or Marmot's own accounts. Authorisation is the [IAM model](access-control.md): roles bound at points in a resource hierarchy, inherited, unioned, with no denies. Machines use [service accounts](access-control.md#service-accounts), one per consumer, with expiring keys. Per-agent audit trails and SIEM export are Enterprise.

<CalloutCard
  title="Reporting a vulnerability"
  description="How to report a vulnerability, what is in scope, and how we respond."
  href="/security"
  buttonText="Security policy"
  variant="external"
  icon="mdi:shield-lock"
/>
