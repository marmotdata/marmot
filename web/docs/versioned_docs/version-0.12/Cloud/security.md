---
sidebar_position: 10
title: Security and isolation
description: How Marmot Cloud keeps instances apart, what an instance can reach and how your data is protected.
---


# Security and isolation

Marmot Cloud stores your catalog's metadata, never your data, and keeps every instance isolated from every other: each has its own database and encryption key, and no instance can reach another.


## What Marmot stores

Marmot stores metadata: asset names, schemas, ownership, descriptions, tags, lineage, glossary terms and statistics.

There are two exceptions:

- **Data queries** let people and agents with the `asset.dataQuerier` role run read-only queries against sources set up to answer them. Marmot returns the rows to the caller without storing them. The query log keeps the statement, who ran it and how many rows came back.
- **Credentials written into a pipeline's `config`** are stored encrypted on your instance. [Secret stores](secret-stores.md) and [keyless pipelines](pipelines.md#how-keyless-pipelines-work) leave nothing to store, which is why we recommend them.

## Isolation

Your instance shares nothing with anyone else's:

- **Your data stays yours.** Each instance has a dedicated database that only your instance can read, so your catalog is never stored alongside another customer's.
- **Your keys are yours.** Each instance has its own encryption key, created with it and destroyed when you delete it.
- **No instance can reach another.** Instances are walled off from each other on the network, so a problem in one can't spread to the next.
- **One way in.** People and software reach your instance only over HTTPS at its address, through the access control you set.

## Outbound traffic

Pipelines read your warehouses and brokers, and people sign in through your identity provider, so your instance can reach public addresses on the internet. It can't reach other instances or Marmot's own infrastructure.

## Limits

Each instance has request and bandwidth limits that grow with its [compute units](instances.md#compute-units), so one busy client can't starve the rest.

## Encryption and backups

Every instance is served over TLS with a certificate that renews automatically. Storage and backups are encrypted at rest, and the credentials Marmot stores are encrypted again with your instance's own key.

Backups are continuous and kept for 14 days. To restore to a point in time within that window, [email support](mailto:support@marmotdata.io) with the instance and the time. A deleted instance [can't be restored](instances.md#deleting-an-instance), and its backups expire within 60 days.

## Sign-in and access

People sign in through [your identity provider](single-sign-on.md), from Team, or with Marmot's own accounts. What they can reach is set by [access control](access-control.md). Software uses [service accounts](service-accounts.md), one per consumer, with keys that can expire. Per-agent audit trails and export to your SIEM are part of Enterprise.

For a security review, [email us](mailto:support@marmotdata.io) and we'll share more detail under NDA.

## Next steps

- [Access control](access-control.md): decide who can see and change what.
- [Security policy](/security): how to report a vulnerability, what's in scope and how we respond.
