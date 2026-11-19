---
title: Webhooks
description: Send a team's notifications to Slack, Discord or any web address.
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Webhooks

A webhook sends a team's notifications to Slack, Discord or any web address, so the team hears about schema changes where it already talks. Each webhook belongs to one team and sends the kinds of notification you choose.

## Add a webhook

1. Open **Settings → Teams**, select the team and open its **Integrations** tab.
2. Select **Add webhook**.
3. Enter a **Name**, choose where to **Send to** (**Slack**, **Discord** or **Web address**) and paste the **Webhook URL**.
4. Under **Notifications to send**, tick the kinds of notification to forward, then select **Add webhook**.

A webhook that sends schema changes to a Discord channel:

<ThemedImg
  lightSrc="/img/new-webhook.png"
  darkSrc="/img/new-webhook-dark.png"
  alt="Add webhook with the name Schema changes to Discord, Discord chosen, a Discord webhook URL and Schema changes and Upstream schema changes ticked"
  size="dialog"
/>

## Check that it worked

Select **Send test** on the webhook. A sample notification arrives in the channel or at the address straight away:

<ThemedImg
  lightSrc="/img/team-webhooks.png"
  darkSrc="/img/team-webhooks-dark.png"
  alt="The Integrations tab of the Data Platform team, listing a Slack webhook with Send test"
  size="panel"
/>

If the endpoint stops accepting notifications, the webhook shows as **Failing**. Fix the endpoint, then send a test.

## Get a webhook URL

### Slack

1. In [Slack apps](https://api.slack.com/apps), create or choose an app.
2. Turn on **Incoming Webhooks** and add a webhook for your channel.
3. Copy the URL, which starts with `https://hooks.slack.com/`.

### Discord

1. Open **Server Settings → Integrations → Webhooks**.
2. Select **New Webhook** and choose the channel.
3. Copy the URL, which starts with `https://discord.com/api/webhooks/`.

### Web address

For your own integrations, Marmot sends a JSON `POST` to any HTTPS address:

```json
{
  "type": "schema_change",
  "title": "Schema Updated",
  "message": "Schema for \"dim_buyer\" has been modified.",
  "data": {
    "link": "/discover/model/dbt/dim_buyer"
  },
  "timestamp": "2026-11-03T10:30:00Z"
}
```

## Next steps

- [Notifications](index.md): what each kind of notification covers.
