---
sidebar_position: 6
title: Notifications
description: Get told when assets you own or follow change, when pipelines finish and when someone mentions you.
---

import { ThemedImg } from '@site/src/components/ThemedImg';

# Notifications

Marmot tells you when assets you own or follow change, when a pipeline finishes and when someone mentions you. The bell in the top bar shows how many are unread and opens **Notifications**, where you can mark them read or delete them:

<ThemedImg
  lightSrc="/img/notifications-page.png"
  darkSrc="/img/notifications-page-dark.png"
  alt="The Notifications inbox with three unread notifications about schema and asset changes and two read ones"
  size="panel"
/>

Read notifications are kept for 14 days.

## Owned and followed assets

You hear about assets your teams own without doing anything. To hear about an asset you don't own, follow it: on the asset's page, select the bell beside its name and choose which changes to hear about:

<ThemedImg
  lightSrc="/img/asset-subscription.png"
  darkSrc="/img/asset-subscription-dark.png"
  alt="The subscription menu on fct_gmv_daily, with Asset Changes and Schema Changes chosen"
  size="panel"
/>

**Manage subscriptions** on **Notifications**, or **Account → Subscriptions**, lists every asset you follow.

## Choose what you hear about

Under **Account → Notifications**, turn each kind of notification on or off for everything you own or follow:

<ThemedImg
  lightSrc="/img/notification-settings.png"
  darkSrc="/img/notification-settings-dark.png"
  alt="Account Notifications, with a switch for each kind of notification from System to Access expiry"
  size="panel"
/>

## Expiring access

Marmot tells you when an API key is about to expire, seven days ahead, and again once it has expired. Notices for your personal keys come to you. Notices for a service account's keys go to whoever created the account, or to everyone who manages service accounts if that person has left. A key created to last seven days or less only gets the second notice, since its short life was chosen on purpose.

Each notice is sent once, and an expired service account key stays listed on the **Settings** overview until it's replaced or removed. Turn these off with **Access expiry**.

## Grouped changes

Marmot groups changes so a bulk update doesn't flood your inbox. Changes to the same assets within 2 minutes arrive as one notification, which is never held back more than 5 minutes.

## Next steps

- [Webhooks](webhooks.md): send a team's notifications to Slack, Discord or any web address.
