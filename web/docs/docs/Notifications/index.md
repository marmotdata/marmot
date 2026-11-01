---
sidebar_position: 6
title: Notifications
description: Stay informed about changes to your data assets with configurable notifications.
---

# Notifications

Marmot keeps you informed about changes across your data catalog. Receive notifications when assets are modified, schemas change, pipelines complete, or someone mentions you in documentation.

import { CalloutCard, DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';
import { TipBox } from '@site/src/components/Steps';

## Viewing Notifications

Click the bell icon in the header to open the notifications panel. Unread notifications appear with a badge showing the count.

<ThemedImg
lightSrc="/img/notifications-unread.png"
darkSrc="/img/notifications-unread-dark.png"
alt="Notifications panel showing unread notifications"

/>

You can click on the bell icon in the header to read your notifications, mark them as read, or, delete them.

<ThemedImg
lightSrc="/img/notifications-page.png"
darkSrc="/img/notifications-page-dark.png"
alt="Full notifications page with filters"

/>

## Subscriptions

Beyond ownership-based notifications, you can subscribe to specific assets to receive notifications regardless of whether you own them. When subscribing, choose which notification types you want for that asset.

<ThemedImg
lightSrc="/img/asset-subscription.png"
darkSrc="/img/asset-subscription-dark.png"
alt="Subscribe button on an asset page"

/>

<TipBox variant="info" title="Subscriptions vs Ownership">
Ownership notifications are automatic - if your team owns an asset, all members receive notifications. Subscriptions let individual users opt in to assets they don't own but want to watch.
</TipBox>

## Preferences

You can control which notification types you receive globally. Navigate to your profile and find the **Notification Preferences** section.

<ThemedImg
lightSrc="/img/notification-settings.png"
darkSrc="/img/notification-settings-dark.png"
alt="Notification preferences panel"

/>

## Expiring Access

Marmot tells you when an API key is about to expire, seven days ahead, and again once it has expired. Notices for your personal keys come to you. Notices for a service account's keys go to whoever created the account, or to everyone who manages service accounts if that person has left. A key created to last seven days or less only gets the second notice, since its short life was chosen on purpose.

Each notice is sent once. Reading or deleting it does not bring it back, and an expired service account key stays listed on the **Settings** overview until it is replaced or removed. Turn them off with the **Access Expiry** preference.

## Aggregation

Marmot batches rapid changes to prevent notification spam. If multiple updates happen to assets you own within a short window, they are grouped into a single notification rather than one per change.

<TipBox variant="info" title="Aggregation Window">
Changes are batched within a 2-minute window, with a maximum 5-minute wait before delivery. This keeps your feed manageable during bulk operations and updates.
</TipBox>

## External Notifications

Send notifications to Slack, Discord, or any HTTP endpoint via team webhooks.

<DocCardGrid>
  <DocCard
    title="Webhooks"
    description="Configure external notifications for your team via Slack, Discord, or generic HTTP webhooks"
    docId="Notifications/webhooks"
    icon="mdi:webhook"
  />
</DocCardGrid>
