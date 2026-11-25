---
title: Create a task
description: Set up a task that answers a question on a schedule or starts when something happens, such as a workload that keeps crashing.
---

# Create a task

Create a task from a ready-made starter or from scratch, choosing when it runs, what it should find out and what it may do. This page builds one that starts when a deployment in `payments` keeps crashing, reads its status and logs, and proposes a restart for 1 person to approve. [Tasks](tasks.md) explains what a task can read and change.

## Before you start

- You hold `task.editor` across the whole catalog.
- For a task that acts on a cluster, a runner serves it. See [Let tasks act on a Kubernetes cluster](../Populating/kubernetes-tools.md).
- Optional: a runbook in **Knowledge** for the task to follow, such as "Payments crash loop".

## Start from a starter

**New task** offers starters for what your instance has: **For your cluster** when a runner reports cluster problems, **For your data** when you can query tables, and **For your catalog** when you can see schemas and pipelines. A starter fills in every step, so you land on **Check the task** with only the name left to decide.

1. Open **Tasks** and select **New task**.
2. Under **For your cluster**, select **Restart crash-looping deployments**.
3. Review each part, select its **Edit** button to change it, then select **Create task**.

To build the same task step by step, select **Start from scratch** instead.

## Choose when it runs

Under **When should this task run?**, choose:

- **Only when someone runs it**, for a task you start yourself.
- **On a schedule**, then **Once** at a date and time, or **Repeating** with **Repeats** and a time.
- **When something happens**, for a task that starts on an event.

For the crash-loop task:

1. Select **When something happens**.
2. Under **Where from**, select `prod-cluster`.
3. Under **What happens**, select **A workload keeps crashing**.
4. Under **Only when**, set **Namespaces** to `payments`.
5. Under **If the same problem is reported again**, select **Start again only after** and set **Wait** to 30 minutes, so one crashing deployment doesn't start the task over and over.
6. Under **If it's already running**, select **Wait, then run once for everything that arrived**.
7. Select **Continue**.

**In short** at the foot of the step says back what you chose.

## Say what it should do

Under **What should it do?**, write the job as you'd brief a colleague: what to look at, what to follow and what to report.

1. Write the job:

   ```text
   A deployment in payments is crash-looping. Check its status and recent
   logs, follow the payments crash-loop runbook, and propose a fix if the
   runbook says one applies.
   ```

2. Optional: under **Runbooks that may help**, add "Payments crash loop". Marmot adds a line asking the task to follow it.
3. Select **Continue**.

## Choose what it may do

**What may it do?** lists what each connected runner offers, with what each tool needs. Leave everything unticked and the task only reads, as the person it runs as.

1. Tick **Read a workload's status**, **Read a pod's logs** and **Read recent events**.
2. Tick **Restart a workload**. It shows **Changes things** and **Needs 1 approval**.
3. Optional: open the row to require more approvals than the runner asks for, or to narrow its parameters.
4. Select **Continue**.

Tools under **Not allowed by the runner** are ones the runner's policy doesn't name. Ask whoever runs the runner to add them.

## Check and create it

1. Under **Check the task**, review each part and select its **Edit** button to change it.
2. Optional: give it a **Name**. Left empty, the task is named after what it does.
3. Select **Create task**, or **Save and try it now** to run it once straight away.

## Check that it worked

Open the task from **Your tasks**. Its page shows when it runs and, under **Tools**, what it may change and what it only looks at. After a run, **Latest result** shows the answer, and **Run history** lists each run. Open a run to see every step the agent took.

When the task proposes a change, it appears under **Waiting for you** on the Tasks page for everyone who can approve it.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| **Where from** shows the runner's source as **Not connected** | The runner isn't connected. Check that it's running and can reach Marmot. |
| **What may it do?** says no source is served from a runner | Set up a runner. See [Let tasks act on a Kubernetes cluster](../Populating/kubernetes-tools.md). |
| A run fails with **Step limit reached** | The job needs more steps than allowed. Make the job narrower, or name the tables or runbook it should use. |
| A run ends **Not completed** | The agent couldn't answer the job as written. Open the run to see why, then select **Edit question**. |
| **Plan limit reached** | Your plan's runs per day or tokens per month are used up. Runs start again the next day, or the next month for tokens. |
| The task's tools are paused | The same target kept coming back after a change. Find out why, then select **Resume tools** on the task's page. |

## Next steps

- [Approve changes](approve-changes.md): review and approve what the task proposes.
- [Start tasks from your alerts](alert-sources.md): connect Alertmanager, Grafana or Datadog.
- [Tasks](tasks.md): what a task can read and change.
