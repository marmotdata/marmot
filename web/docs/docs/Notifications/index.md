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
    href="/docs/Notifications/webhooks"
    icon="mdi:webhook"
  />
</DocCardGrid>
