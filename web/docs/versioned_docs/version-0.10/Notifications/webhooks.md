---
title: Webhooks
description: Send team notifications to Slack, Discord, or any HTTP endpoint.
---

# Webhooks

Send your team's asset notifications to external services. Each webhook is scoped to a team and can be configured to forward specific notification types.

import { FeatureCard, FeatureGrid } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';
import { Steps, Step, TipBox } from '@site/src/components/Steps';

## Supported Providers

<FeatureGrid>
  <FeatureCard
    title="Slack"
    description="Rich formatted messages to any Slack channel via incoming webhooks"
    icon="mdi:slack"
  />
  <FeatureCard
    title="Discord"
    description="Embedded notifications to Discord channels via webhook URLs"
    icon="simple-icons:discord"
  />
  <FeatureCard
    title="Generic HTTP"
    description="JSON payloads to any HTTP endpoint for custom integrations"
    icon="mdi:webhook"
  />
</FeatureGrid>

## Setup

<Steps>
<Step title="Navigate to Team Settings">

Open your team page and find the **Webhooks** section.

<ThemedImg
lightSrc="/img/team-webhooks.png"
darkSrc="/img/team-webhooks-dark.png"
alt="Team webhooks section"

/>

</Step>
<Step title="Create a Webhook">

Click **Add Webhook** and fill in the details:

- **Name** — a descriptive label (e.g. "Schema alerts to #data-eng")
- **Provider** — choose Slack, Discord, or Generic
- **Webhook URL** — the incoming webhook URL from your provider
- **Notification Types** — select which types to forward

<ThemedImg
lightSrc="/img/new-webhook.png"
darkSrc="/img/new-webhook-dark.png"
alt="Create webhook form"

/>

</Step>
<Step title="Test the Webhook">

Click **Send Test** to verify the webhook is configured correctly. A sample notification will be delivered to your endpoint.

<!-- TODO: Add screenshot of test notification in Slack/Discord -->

</Step>
</Steps>

## Provider Details

### Slack

Create an incoming webhook in your Slack workspace:

1. Go to [Slack Apps](https://api.slack.com/apps) and create or select an app
2. Enable **Incoming Webhooks** and add a new webhook to your channel
3. Copy the webhook URL (starts with `https://hooks.slack.com/`)

Messages are formatted with rich blocks showing the notification type, affected asset, and a link back to Marmot.

### Discord

Create a webhook in your Discord server:

1. Open **Server Settings** > **Integrations** > **Webhooks**
2. Click **New Webhook** and select the target channel
3. Copy the webhook URL (starts with `https://discord.com/api/webhooks/`)

Notifications are delivered as embedded messages with colour-coded types.

### Generic HTTP

For custom integrations, the generic provider sends a JSON POST to any HTTPS endpoint:

```json
{
  "type": "schema_change",
  "title": "Schema changed on users_table",
  "message": "Column 'email' type changed from VARCHAR to TEXT",
  "asset_mrn": "postgres://prod/public/users_table",
  "team_id": "d4e5f6...",
  "timestamp": "2025-01-23T10:30:00Z"
}
```
