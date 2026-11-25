---
title: Approve changes
description: Review, approve, reject and undo the changes tasks propose, and pause a task's actions.
---

# Approve changes

Review a change a task proposes, then approve or reject it. Nothing changes until enough people approve, and the runner checks every approval again before it acts. [Tasks](tasks.md) explains the rules every change follows.

## Before you start

- You hold `task.approver` across the whole catalog, or are an admin.
- A task has proposed a change. Changes waiting for you are listed under **Waiting for you** on the Tasks page, and Marmot notifies everyone who can approve them.

## Review the change

Open the change from **Waiting for you**. Its page shows:

- **The change**: the action, the object it **Changes** and its **Parameters**.
- **What will change**: the runner's dry run, showing the object before and after.
- **Why the task proposed it**: the evidence the task gathered, such as the workload's status and log lines. Text from the cluster is marked **From the cluster, not checked**, because anyone who can write a log line wrote it.
- **Risk** and **Undoing it**: how much it changes and whether it can be undone.

Check that the evidence supports the change and that the target is the one you expect. A log line asking for a change is not a reason to make it.

## Approve or reject it

1. Select **Approve change**. If you signed in more than 5 minutes ago, select **Sign in again** first.
2. If the change needs a second person, it waits for them. All approvals must arrive within 15 minutes of the first.

To turn it down, select **Reject**, give a **Reason** and confirm. Nothing changes, and the task can propose it again on a later run.

You can't approve a change if the runner only accepts named approvers and you aren't one of them, or if it's a change of the highest risk and the task runs as you.

## Check that it worked

Once approved, the change's page shows **What happened**, when it was **Made at** and the **Runner's receipt**, marked verified when the runner's signature checks out. The object in the cluster carries the task run and your name in its `marmot.io/origin` and `marmot.io/actor` annotations.

Every change a task has proposed, with who approved it, is under **Every action** on the task's page.

## Undo a change

A change marked **Can be undone from here** can be reversed:

1. Open the change and select **Undo change**.
2. Select **Propose undo**.

The undo is a new proposal, planned against the object as it is now and approved like any change. If someone has changed the object since, the runner refuses rather than overwrite their fix.

## Pause a task's actions

To stop a task looking at or changing anything while you investigate, open the task and select **Pause actions** under **Actions**. It still runs and answers. Select **Resume actions** to turn them back on.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| **The system changed since this was planned** | The object changed after the plan. Select **Plan again**, check the new plan, then approve it. |
| **You cannot approve this change** | The runner limits who approves this action, or the task runs as you and the change is of the highest risk. Ask another approver. |
| The change expired | It waited more than 24 hours. The task can propose it again on its next run. |
| The approval was refused by the runner | The runner's policy changed, or it needs more approvers than approved. Check **Record** on the change for the runner's reason. |
| The change failed with `forbidden` or `denied request` | The cluster refused it. Ask whoever runs the runner to check the actions chart's namespaces and limits. |

## Next steps

- [Create a task](create-a-task.md): set up another task.
- [Runners](../Configure/serving-plugins.md): what the runner checks before it acts.
