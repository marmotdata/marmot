---
title: Start tasks from your alerts
description: Connect Alertmanager, Grafana or Datadog so an alert can start a task.
---

# Start tasks from your alerts

Connect an alerting system as an alert source, so an alert firing or resolving can start a [task](tasks.md). Your alerting system sends alerts to an address Marmot gives you, with a secret that proves they came from it. Alertmanager, Grafana and Datadog are supported.

## Before you start

- You hold `task.editor` and manage access (`iam.admin`) across the whole catalog, because an alert source can start any task.
- You can add a webhook in your alerting system.

## Add the alert source

1. Open **Settings → Tasks** and select **Add alert source**.
2. Enter a **Name**, such as `Production Alertmanager`, and choose the **Alerting system**.
3. Select **Create alert source**.
4. Copy the **Address** and the **Secret**. Marmot can't show the secret again.
5. Select **I have copied these**.

## Send alerts to it

Add a webhook in your alerting system with the address, and send the secret the way that system supports:

| Alerting system | Where the secret goes |
| --- | --- |
| Alertmanager | A webhook receiver with the address, and its authorisation credentials set to the secret |
| Grafana | A webhook contact point with the address, and its HMAC secret set to the secret |
| Datadog | A webhook with the address, and a custom header `X-Marmot-Webhook-Secret` set to the secret |

For Alertmanager, the receiver looks like this, with the secret in a file:

```yaml
receivers:
  - name: marmot
    webhook_configs:
      - url: <address>
        http_config:
          authorization:
            credentials_file: /etc/alertmanager/marmot-secret
```

## Start a task from it

In a task's **When should this task run?**, select **When something happens**, then:

1. Under **Where from**, select the alert source under **Alerts**.
2. Under **What happens**, select **An alert starts firing** or **An alert resolves**.
3. Optional: under **Only when**, narrow it by **Severities** or **Labels**.

The same problem means the same alert, with the same labels, from the same alert source. **If the same problem is reported again** decides whether a repeat starts the task.

## Check that it worked

Fire a test alert from your alerting system. The task starts a run, shown under **Running now** on the Tasks page, and the run's page names the alert under **Alert was about**.

## Replace or delete the secret

If the secret may have leaked, open the alert source's **More actions** menu and select **Replace secret**. The old address and secret stop working at once, so update the alerting system straight away.

**Delete alert source** stops its address working at once, and tasks that name it stop hearing from it.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| No run starts | Check the webhook's address and secret, and that the task's **Only when** matches the alert's labels and severity. |
| Runs start for every repeat of an alert | Under **If the same problem is reported again**, select **Start again only after** and set a **Wait**. |
| **Events are held** on the task's page | Many alerts arrived at once. Choose whether to run the task for them or discard them. |

## Next steps

- [Create a task](create-a-task.md): set up a task that starts on an alert.
- [Approve changes](approve-changes.md): act on what the task proposes.
