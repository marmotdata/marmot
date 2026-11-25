---
sidebar_position: 9
title: Runners
description: How a runner serves queries, reads, tools and triggers from your own network, what it decides for itself and what Marmot can never make it do.
---

# Runners

A runner is a Marmot plugin you run inside your own network, next to a database or a Kubernetes cluster, so Marmot can query it without ever holding its credentials. The runner always connects out to Marmot, and Marmot never connects into your network. It's the same `marmot plugin` command you use to ingest, started with `--serve`.

To set one up, see [Serve queries from your network](../Populating/serve-queries.md) or [Let tasks act on a Kubernetes cluster](../Populating/kubernetes-tools.md). Every flag and policy key is in the [runner reference](serving-reference.md).

## How it works

<figure className="doc-diagram" role="img" aria-label="The runner sits inside your network, next to your database and your Kubernetes cluster. It opens one connection out to Marmot and connects to the database and the cluster with credentials that stay in your network. Marmot sends signed requests down the runner's connection and the runner sends back signed answers, receipts and events. Nothing connects into your network.">
  <svg viewBox="0 0 640 220" xmlns="http://www.w3.org/2000/svg">
    <rect className="doc-diagram__box" x="8" y="10" width="400" height="200" rx="8" />
    <text className="doc-diagram__label" x="24" y="34">Your network</text>
    <rect className="doc-diagram__box doc-diagram__box--root" x="150" y="90" width="120" height="44" rx="6" />
    <text className="doc-diagram__label" x="210" y="117" textAnchor="middle">Runner</text>
    <rect className="doc-diagram__box doc-diagram__box--child" x="24" y="56" width="96" height="40" rx="6" />
    <text className="doc-diagram__label doc-diagram__label--child" x="72" y="81" textAnchor="middle">Database</text>
    <rect className="doc-diagram__box doc-diagram__box--child" x="24" y="140" width="96" height="40" rx="6" />
    <text className="doc-diagram__label doc-diagram__label--child" x="72" y="165" textAnchor="middle">Cluster</text>
    <path className="doc-diagram__line" d="M150 104 H120 M150 120 H120 V160" />
    <rect className="doc-diagram__box doc-diagram__box--root" x="500" y="90" width="132" height="44" rx="6" />
    <text className="doc-diagram__label" x="566" y="117" textAnchor="middle">Marmot</text>
    <path className="doc-diagram__line" d="M270 112 H500 M490 106 L500 112 L490 118" />
    <text className="doc-diagram__label" x="385" y="102" textAnchor="middle">connects out</text>
  </svg>
</figure>

1. The runner starts, ingests as usual and enrols with Marmot over HTTPS, using an ordinary Marmot credential that holds **Serve ingestion** (`ingestion:serve`).
2. Marmot issues it a short-lived certificate, and the runner opens one long-running connection to Marmot's serve port with mutual TLS.
3. When a person, an agent or a task queries the source, Marmot checks their access, then signs the exact request and sends it down that connection.
4. The runner checks the signature and its own policy, runs the request with credentials only it holds, and answers with a signed receipt.

The runner listens on no port. It only needs to reach Marmot and the systems it serves.

## What a runner can serve

A runner serves 1 source, named after its pipeline. Depending on the plugin and on what you turn on, that source can offer:

- **Queries.** Read-only SQL against a database, for people, agents and [tasks](../Cloud/tasks.md). Every plugin with `queryable: true` supports this.
- **Reads.** Typed lookups that change nothing, such as a Kubernetes workload's status, a pod's logs or recent events, so a task can see what's wrong.
- **Tools.** Typed changes with bounded parameters, such as restarting a deployment or suspending a CronJob. Each is previewed first and runs only once people approve it.
- **Triggers.** Watches the runner keeps on the system, such as "a workload keeps crashing", which start a task when they fire.

Queries are on as soon as the runner serves. Reads, tools and triggers are off until the runner is started with `--allow-tools` and its policy file names each one.

## What the runner decides

Marmot asks and the runner decides. Each layer can only narrow what the one before allows:

| Layer | Owned by | What it limits |
| --- | --- | --- |
| Access in Marmot | Your Marmot admins | Who may query the source, and which tools a task may propose and on what |
| The runner's policy file | Whoever runs the runner | Which sources, tools, parameter values, triggers, actors and origins are served at all, and who must approve |
| The credentials the runner holds | Whoever runs the runner | Everything the database account or Kubernetes service account can't do |
| The cluster's admission policy | Whoever runs the cluster | Any change beyond the exact fields a tool touches |

Marmot can't widen any of it. The policy, the approvers and the triggers live only on the runner, and Marmot never sends a trigger definition, a command or an expression. A model in a task never writes a command either: it picks a tool the task was given and fills in typed parameters, which the runner checks again.

## Approvals

Every tool that changes something needs approval, and the runner checks the approval itself before it runs anything.

People approve a change in Marmot. They need the `task:approve` permission and must have signed in within the last 5 minutes. A change of the highest risk needs 2 different people, and never the person the task runs as. Marmot then signs the approval with an approval key used for nothing else.

The runner recomputes what is being approved from the request in front of it: the tool, its parameters, the state it was planned against and who approved. It accepts an approval once, within its time limit, and only with as many distinct approvers as its own policy asks for. If the system changed since the plan, the runner refuses and the change must be planned again.

Approvals are signed by Marmot, not by each person. Someone who took over Marmot along with its approval key could approve changes, but only within each runner's policy, its credentials and the cluster's admission policy, and every one would be in the runner's own audit log. The runner logs `approvals: Marmot-signed` when it starts and on every receipt, so nobody mistakes it for a person's own signature.

## Audit and receipts

Every query and tool ends with a receipt signed by the runner, which Marmot checks and stores. **Query audit** shows each query's receipt as `verified`, `invalid` or `missing`, and a task's **Tool history** shows the receipt for each change.

With `--audit-log`, the runner also keeps its own record: one JSON line per query, read, plan, tool and trigger event, each line chained to the one before by its hash. Editing or removing a line breaks the chain, which `marmot ingest verify-audit-log` reports. Neither side can rewrite history alone. A tool is written to the log as an intent before it runs, and if the runner can't write the log, nothing runs.

Every change a tool makes in Kubernetes is also stamped on the object with the task run that proposed it and the person who approved it.

## Trust and keys

The runner learns which keys to trust from its first enrolment, and from then on only accepts new keys Marmot announces alongside ones it already trusts. Pass `--ca-fingerprint` to check that first enrolment too.

- **Certificates last 10 minutes** and are renewed with a fresh key, so a stolen certificate stops working within minutes.
- **Every request is signed** for that one connection, expires after a minute and can be used once, so it can't be replayed or moved to another runner.
- **A source belongs to the credential that first served it**, so another credential can't take it over and receive its queries.
- **Revoking the runner's credential**, its service account or its **Serve ingestion** permission cuts it off within 15 seconds.

Self-hosted servers can keep the signing keys in a KMS, so a copy of Marmot's database is not enough to impersonate it. See [Rotate and protect the keys](../Populating/serve-queries.md#rotate-and-protect-the-keys).

## Limitations

- A runner serves 1 queryable run per pipeline. Split other sources into their own pipelines and runners.
- Results are capped at 256 MiB for queries and 256 KiB for reads.
- Objects managed by Flux or Argo CD are refused by Kubernetes tools, because their changes belong in Git and the next reconcile would revert them.
- Approvals signed by your own identity provider are not offered. Approvals are Marmot-signed on every plan.
- The serve port carries mutual TLS end to end, so it can't sit behind a proxy or Ingress that terminates TLS.

## Next steps

- [Serve queries from your network](../Populating/serve-queries.md): run your first runner next to a database.
- [Let tasks act on a Kubernetes cluster](../Populating/kubernetes-tools.md): add reads, tools and triggers.
- [Runner reference](serving-reference.md): every flag, server setting and policy key.
