---
sidebar_position: 6
---

import { CalloutCard, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Notifications

Marmot keeps you informed about changes that matter to your data assets. When someone modifies an asset you own, mentions you in documentation or completes a pipeline job, you receive a notification so you can stay on top of what's happening across your catalog.

<CalloutCard
  title="Try It Out"
  description="See notifications in action with the interactive demo."
  href="https://demo.marmotdata.io"
  buttonText="View Demo"
  icon="mdi:rocket-launch"
/>

## Notification Types

Marmot supports several notification types to keep you informed without overwhelming you with noise.

<FeatureGrid>
  <FeatureCard
    title="Schema Changes"
    description="Get notified when the schema of an asset you own is modified"
    icon="mdi:table-edit"
  />
  <FeatureCard
    title="Asset Changes"
    description="Receive updates when metadata changes on assets you own"
    icon="mdi:database-edit"
  />
  <FeatureCard
    title="Mentions"
    description="Know when someone mentions you or your team in documentation"
    icon="mdi:at"
  />
  <FeatureCard
    title="Job Completion"
    description="Track when pipeline jobs finish running"
    icon="mdi:check-circle"
  />
</FeatureGrid>

## Viewing Notifications

Click the bell icon in the header to open the notifications panel. Unread notifications appear with a badge showing the count. From the panel you can mark notifications as read, delete them or click through to the relevant asset or page.

<!-- TODO: Add screenshot -->
<!-- <ThemedImg
  lightSrc="/img/notifications-panel-light.png"
  darkSrc="/img/notifications-panel-dark.png"
  alt="Notifications panel"
/> -->

For a full view of all your notifications, click **View All** to open the dedicated notifications page. Here you can filter by read status and manage notifications in bulk.

<!-- TODO: Add screenshot -->
<!-- <ThemedImg
  lightSrc="/img/notifications-page-light.png"
  darkSrc="/img/notifications-page-dark.png"
  alt="Notifications page"
/> -->

## Configuring Preferences

You control which notifications you receive. Navigate to your profile by clicking your avatar in the header, then find the **Notification Preferences** section.

<!-- TODO: Add screenshot -->
<!-- <ThemedImg
  lightSrc="/img/notifications-preferences-light.png"
  darkSrc="/img/notifications-preferences-dark.png"
  alt="Notification preferences"
/> -->

Each notification type can be toggled independently:

| Type | Description |
| ---- | ----------- |
| **System** | Announcements and platform updates from Marmot |
| **Schema Changes** | Modifications to the schema of assets you or your team own |
| **Asset Changes** | Metadata updates to assets you or your team own |
| **Mentions** | When someone @mentions you or your team in documentation |
| **Job Completion** | When pipeline runs finish executing |

Toggle off any notification types you don't need. Your preferences are saved automatically.

## How Ownership Works

Asset notifications are sent to the owners of an asset. Ownership is determined by team assignments. When a team is assigned as an owner of an asset, all members of that team receive notifications for changes to that asset.

To assign ownership:

1. Navigate to the asset page
2. Find **Owners** in the sidebar
3. Click **Add** and search for a team
4. Select the team to assign ownership

## Aggregation

To prevent notification spam, Marmot aggregates rapid changes. If multiple updates happen to assets you own within a short window, they are batched into a single notification rather than sending one for each change. This keeps your notification feed manageable during bulk operations or automated updates.

<CalloutCard
  title="Need Help?"
  description="Join the Discord community to ask questions and share feedback about notifications."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
