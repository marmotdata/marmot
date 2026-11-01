<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { setCrumbs } from '$lib/mm';
	import { notifications, type Notification } from '$lib/stores/notifications';
	import {
		renderNotificationTitle,
		renderNotificationMessage,
		notificationPath
	} from '$lib/notifications/copy';
	import '@marmotdata/elements/notification-list.js';
	import type {
		NotificationType,
		NotificationReadDetail
	} from '@marmotdata/elements/classes/notification-list.js';

	const KNOWN: NotificationType[] = [
		'system',
		'schema_change',
		'asset_change',
		'team_invite',
		'mention',
		'job_complete',
		'upstream_schema_change',
		'downstream_schema_change',
		'lineage_change',
		'asset_deleted',
		'access_expiry'
	];

	setCrumbs([{ label: m.notif_heading() }]);

	const inbox = $derived($notifications);
	const list = $derived(inbox.notifications);
	const hasRead = $derived(list.some((n) => n.read));
	const firstLoad = $derived(inbox.loading && list.length === 0);
	const empty = $derived(!inbox.loading && !inbox.error && list.length === 0);

	onMount(() => {
		notifications.refresh();
	});

	function typeOf(n: Notification): NotificationType {
		return KNOWN.includes(n.type as NotificationType) ? (n.type as NotificationType) : 'system';
	}

	function hrefOf(n: Notification): string {
		const path = notificationPath(n);
		return path ? resolve(path as '/') : '';
	}

	function messageOf(n: Notification): string {
		const message = renderNotificationMessage(n);
		return n.recipient_type === 'team' ? `${message} · ${m.notif_via_team()}` : message;
	}

	function onActivate(n: Notification) {
		if (!n.read) notifications.markAsRead(n.id);
	}

	function onRead(e: CustomEvent<NotificationReadDetail>) {
		if (e.detail.read) notifications.markAsRead(e.detail.id);
		else notifications.markAsUnread(e.detail.id);
	}

	// Pruning removes read notifications after this long, so the page says so before they disappear
	const retention = $derived(
		inbox.summary.read_retention_days
			? m.notif_read_retention({ count: inbox.summary.read_retention_days })
			: ''
	);
</script>

<div class="mm-stack page">
	<mm-page-header heading={m.notif_heading()} description={retention}>
		<mm-button slot="actions" icon="tune" href={resolve('/account/subscriptions')}
			>{m.notif_manage_subscriptions()}</mm-button
		>
	</mm-page-header>

	{#if empty}
		<mm-panel heading={m.notif_inbox()} icon="notifications" empty>
			<mm-empty-state
				slot="empty"
				compact
				seed={8}
				heading={m.notif_empty_heading()}
				description={m.notif_empty_all()}
			>
				<mm-button slot="action" href={resolve('/discover')} icon="travel-explore"
					>{m.notif_find_assets()}</mm-button
				>
			</mm-empty-state>
		</mm-panel>
	{:else}
		<mm-panel
			heading={m.notif_inbox()}
			icon="notifications"
			count={inbox.error ? undefined : inbox.summary.total_count || undefined}
		>
			{#if hasRead}
				<mm-button
					slot="actions"
					size="sm"
					variant="danger"
					icon="delete"
					onclick={() => notifications.clearRead()}>{m.notif_clear_read()}</mm-button
				>
			{/if}
			<mm-notification-list
				label={m.notif_heading()}
				filters
				loading={firstLoad}
				error={inbox.error ?? ''}
				onmm-retry={() => notifications.refresh()}
				onmm-read-all={() => notifications.markAllAsRead()}
			>
				{#each list as n (n.id)}
					<mm-notification-item
						notification-id={n.id}
						type={typeOf(n)}
						heading={renderNotificationTitle(n)}
						message={messageOf(n)}
						href={hrefOf(n)}
						time={n.created_at}
						read={n.read}
						onmm-activate={() => onActivate(n)}
						onmm-read={onRead}
						onmm-delete={() => notifications.deleteNotification(n.id)}
					></mm-notification-item>
				{/each}
			</mm-notification-list>
			{#if inbox.hasMore && list.length > 0}
				<div slot="footer">
					<mm-button
						size="sm"
						icon="expand-more"
						disabled={inbox.loading}
						onclick={() => notifications.loadMore()}>{m.notif_load_more()}</mm-button
					>
				</div>
			{/if}
		</mm-panel>
	{/if}
</div>

<style>
	.page {
		max-width: 60rem;
	}
</style>
