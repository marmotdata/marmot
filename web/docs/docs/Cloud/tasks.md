---
title: Tasks
description: What tasks are, what starts them, what they can read and change, and how every answer and change is traced.
---

# Tasks

A task is a job Marmot's agent does for you in the background: a question it answers on a schedule, or a check it runs when something happens, such as a schema change, a failed pipeline, an alert or a crash-looping deployment. Each run reads your catalog and data with the access of the person or service account it runs as, and answers with figures you can trace back to the query that produced them. Given a [runner](../Configure/serving-plugins.md), a task can also look at a Kubernetes cluster and propose fixes, which only happen once people approve them.

To make one, see [Create a task](create-a-task.md).

## How it works

1. Something starts the task: a person, a schedule or an event.
2. The agent works through the job in steps: it searches the catalog, reads runbooks and knowledge, runs read-only queries and, with a runner, reads a cluster.
3. It answers with a result. Each figure in the result links to the query, row and column it came from, and the result lists the knowledge it rests on.
4. If the task may change things, it can also propose a change. The change waits under **Waiting for you** until enough people approve it, then the runner makes it.

Every step is recorded on the run's page, including each query and what the agent read.

## What starts a task

You choose one when you create the task:

- **Only when someone runs it.** Start it from the task's page with **Run now**.
- **On a schedule.** Once at a date and time, or repeating hourly, daily, on weekdays, weekly or monthly, or with a cron expression.
- **When something happens.** An event from one of these sources:

| Source | Events |
| --- | --- |
| Catalogue | An asset is added or removed, its schema changes, its description, owners or tags change, its lineage changes |
| Pipelines | An ingestion run finishes, with any result, succeeded or failed |
| A runner | Whatever its plugin and policy report, such as "A workload keeps crashing" or "A node is unhealthy" |
| Alerts | An alert starts firing or resolves, from an [alert source](alert-sources.md) |

You can narrow an event with **Only when**, for example to one namespace or a name prefix. You also choose what happens when the same problem is reported again, and when an event arrives while the task is already running. Whatever starts a task, anyone who may manage it can always run it with **Run now**.

## Whose access a task uses

A task runs as the person who last saved it, or as a service account they chose. It can only read what that identity may read and query only the sources it may query. Saving a task hands it to whoever saves it, so another editor's save changes whose access it uses; the edit page says so before you save.

Events don't widen this. A task started by an alert still reads only what its owner may read.

## What a task may change

A task only reads unless you give it actions under **What may it do?**. The list comes from connected runners: each offers the actions its policy allows, with how many approvals each needs. Leave everything unticked and the task never changes anything.

A task given actions can:

- **Look** with reads, such as a workload's status, a pod's logs or recent events. Reads need no approval.
- **Propose a change**, such as restarting a workload. It never makes the change itself.

For a task started by a cluster problem or an alert, a change is held to the object that started the run, so a task woken by one crashing deployment can't propose changes to another.

## Approving changes

A proposed change shows exactly what will change, the plan the runner made and the evidence the task gathered. It runs only once enough people approve it, signed in within the last 5 minutes. The runner checks the approval again before anything changes. See [Approve changes](approve-changes.md).

These rules apply to every task:

| Rule | Value |
| --- | --- |
| A proposal waits for approval for | 24 hours |
| All approvals must arrive within | 15 minutes of the first, or approving starts again |
| Most changes one run may propose | 3 |
| Times an alert may return before the task's actions pause | 2 |

The last rule stops a task from restarting the same thing over and over. When the same target keeps coming back after a change, the task's actions pause until someone resumes them.

## Who can do what

Grant these roles across the whole catalog. Admins hold all of them.

| Role | Gives |
| --- | --- |
| `task.viewer` | See tasks, their runs and their usage |
| `task.reader` | `task.viewer`, plus read results and the record of what the agent did |
| `task.editor` | Create, edit, delete and run tasks, and read their results |
| `task.approver` | `task.reader`, plus approve or reject the changes tasks propose |

Setting up [alert sources](alert-sources.md) needs someone who manages access as well as tasks, because an alert source can start any task.

## Limits

Each run stops when it reaches one of its limits, shown under **Details** on the task's page:

| Limit | Default |
| --- | --- |
| Steps | 25 |
| Queries | 20 |
| Rows per query | 200 |
| Tokens | 300,000 |
| Time limit | 10 minutes |

Your plan also limits runs per day and tokens per month. **Plan** appears on the Tasks page when you're close to either.

## Limitations

- A task reads your catalog, data and runners. It can't browse the web or reach systems no runner serves.
- Changes go through runners only. Kubernetes is the only plugin with actions.
- Every change needs at least 1 approval. Nothing runs unattended.
- Objects managed by Flux or Argo CD can't be changed, because the next reconcile would revert them.

## Next steps

- [Create a task](create-a-task.md): set one up from a starter or from scratch.
- [Approve changes](approve-changes.md): review, approve, reject and undo what tasks propose.
- [Let tasks act on a Kubernetes cluster](../Populating/kubernetes-actions.md): connect a cluster.
