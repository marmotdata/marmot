<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import { notifications, type Notification } from '$lib/stores/notifications';
	import { formatRelativeTime, formatList } from '$lib/utils/format';
	import { m } from '$lib/paraglide/messages';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import SegmentedControl from '$components/ui/SegmentedControl.svelte';
	import { BTN_GHOST, BTN_SECONDARY } from '$lib/ui/classes';
	import { accessExpiryCopy } from '$lib/notifications/accessExpiry';

	type Filter = 'all' | 'unread' | 'read';
	let filter = $state<Filter>('all');

	let notifState = $derived($notifications);
	let filteredNotifications = $derived(
		notifState.notifications.filter((n) => {
			if (filter === 'unread') return !n.read;
			if (filter === 'read') return n.read;
			return true;
		})
	);

	onMount(() => {
		notifications.refresh();
	});

	function getNotificationIcon(notification: Notification): string {
		switch (notification.type) {
			case 'asset_change':
				return 'material-symbols:database';
			case 'schema_change':
			case 'upstream_schema_change':
			case 'downstream_schema_change':
				return 'material-symbols:schema';
			case 'lineage_change':
				return 'material-symbols:account-tree';
			case 'asset_deleted':
				return 'material-symbols:delete';
			case 'team_invite':
				return 'material-symbols:group-add';
			case 'mention':
				return 'material-symbols:alternate-email';
			case 'access_expiry':
				return accessExpiryCopy(notification).expired
					? 'material-symbols:timer-off-outline'
					: 'material-symbols:timer-outline';
			case 'job_complete':
				if (notification.data?.status === 'failed') return 'material-symbols:error';
				if (notification.data?.status === 'cancelled') return 'material-symbols:cancel';
				return 'material-symbols:check-circle';
			case 'system':
			default:
				return 'material-symbols:info';
		}
	}

	// Colour only marks a state worth noticing (a failure, a deletion, a cancelled run); every other kind sits on a neutral tile with its own icon
	function getNotificationColors(notification: Notification): { bg: string; icon: string } {
		const danger = { bg: 'bg-red-50 dark:bg-red-950/40', icon: 'text-red-600 dark:text-red-400' };
		const warning = {
			bg: 'bg-amber-50 dark:bg-amber-950/40',
			icon: 'text-amber-600 dark:text-amber-400'
		};
		const neutral = {
			bg: 'border border-gray-100 bg-gray-50 dark:border-gray-700 dark:bg-gray-900',
			icon: 'text-gray-500 dark:text-gray-400'
		};
		if (notification.type === 'asset_deleted') return danger;
		if (notification.type === 'access_expiry') {
			return accessExpiryCopy(notification).expired ? danger : warning;
		}
		if (notification.type === 'job_complete') {
			if (notification.data?.status === 'failed') return danger;
			if (notification.data?.status === 'cancelled') return warning;
			return {
				bg: 'bg-earthy-green-50 dark:bg-earthy-green-900/30',
				icon: 'text-earthy-green-700 dark:text-earthy-green-400'
			};
		}
		return neutral;
	}

	// Copy is rebuilt from the stored type and data so it translates, falling back to the server's English for old rows and unknown types
	function text(data: Record<string, unknown> | undefined, key: string): string | null {
		const value = data?.[key];
		return typeof value === 'string' && value.length > 0 ? value : null;
	}

	function count(data: Record<string, unknown> | undefined, key: string): number | null {
		const value = data?.[key];
		return typeof value === 'number' ? value : null;
	}

	function isBatched(notification: Notification): boolean {
		return count(notification.data, 'asset_count') !== null;
	}

	function renderNotificationTitle(notification: Notification): string {
		const batched = isBatched(notification);
		switch (notification.type) {
			case 'schema_change':
				return m.notif_schema_change_title();
			case 'upstream_schema_change':
				return m.notif_upstream_schema_change_title();
			case 'downstream_schema_change':
				return m.notif_downstream_schema_change_title();
			case 'lineage_change':
				return batched ? m.notif_batch_lineage_change_title() : m.notif_lineage_change_title();
			case 'asset_deleted':
				return batched ? m.notif_batch_asset_deleted_title() : m.notif_asset_deleted_title();
			case 'asset_change':
				return batched ? m.notif_batch_asset_change_title() : m.notif_asset_change_title();
			case 'job_complete':
				switch (text(notification.data, 'status')) {
					case 'failed':
						return m.notif_job_failed_title();
					case 'cancelled':
						return m.notif_job_cancelled_title();
					case 'completed':
						return m.notif_job_complete_title();
					default:
						return notification.title;
				}
			case 'access_expiry':
				return accessExpiryCopy(notification).title;
			case 'team_invite':
				return m.notif_team_invite_title();
			case 'mention':
				return text(notification.data, 'mention_target') === 'team'
					? m.notif_mention_team_title()
					: m.notif_mention_user_title();
			default:
				return notification.title;
		}
	}

	function renderNotificationMessage(notification: Notification): string {
		const data = notification.data;
		const assetName = text(data, 'asset_name');
		const assetCount = count(data, 'asset_count');

		switch (notification.type) {
			case 'schema_change':
				return assetName
					? m.notif_schema_change_message({ name: assetName })
					: notification.message;
			case 'upstream_schema_change':
				return assetName
					? m.notif_upstream_schema_change_message({ name: assetName })
					: notification.message;
			case 'downstream_schema_change':
				return assetName
					? m.notif_downstream_schema_change_message({ name: assetName })
					: notification.message;
			case 'lineage_change':
				if (assetCount !== null) return m.notif_batch_lineage_change_message({ count: assetCount });
				return assetName
					? m.notif_lineage_change_message({ name: assetName })
					: notification.message;
			case 'asset_deleted':
				if (assetCount !== null) return m.notif_batch_asset_deleted_message({ count: assetCount });
				return assetName
					? m.notif_asset_deleted_message({ name: assetName })
					: notification.message;
			case 'asset_change': {
				const eventCount = count(data, 'count');
				if (assetCount !== null && eventCount !== null) {
					return m.notif_batch_asset_change_message({ count: eventCount, assetCount });
				}
				const fields = data?.changed_fields;
				if (assetName && Array.isArray(fields) && fields.length > 0) {
					return m.notif_asset_change_message({
						fields: formatList(fields.map(String)),
						name: assetName
					});
				}
				return notification.message;
			}
			case 'job_complete': {
				const pipeline = text(data, 'pipeline_name');
				if (!pipeline) return notification.message;
				switch (text(data, 'status')) {
					case 'completed': {
						const entities = count(data, 'total_entities');
						return entities !== null
							? m.notif_job_complete_message_entities({ name: pipeline, count: entities })
							: m.notif_job_complete_message({ name: pipeline });
					}
					case 'failed': {
						const error = text(data, 'error_message');
						return error
							? m.notif_job_failed_message_reason({ name: pipeline, error })
							: m.notif_job_failed_message({ name: pipeline });
					}
					case 'cancelled':
						return m.notif_job_cancelled_message({ name: pipeline });
					default:
						return notification.message;
				}
			}
			case 'access_expiry':
				return accessExpiryCopy(notification).message;
			case 'team_invite': {
				const teamName = text(data, 'team_name');
				const role = text(data, 'role');
				return teamName && role
					? m.notif_team_invite_message({ name: teamName, role })
					: notification.message;
			}
			case 'mention': {
				const actor = text(data, 'mentioner');
				const page = text(data, 'page_title');
				if (!actor || !page) return notification.message;
				return text(data, 'mention_target') === 'team'
					? m.notif_mention_team_message({ actor, page })
					: m.notif_mention_user_message({ actor, page });
			}
			default:
				return notification.message;
		}
	}

	async function handleNotificationClick(notification: Notification) {
		if (!notification.read) {
			await notifications.markAsRead(notification.id);
		}

		// Deleted assets no longer exist, so don't try to navigate
		if (notification.type === 'asset_deleted') {
			return;
		}

		if (notification.data?.link) {
			goto(resolve(notification.data.link as `/${string}`));
		} else if (notification.data?.asset_mrn) {
			goto(
				resolve(
					`/discover/${encodeURIComponent(notification.data.asset_mrn as string)}` as `/${string}`
				)
			);
		} else if (notification.data?.team_id) {
			goto(resolve(`/settings/teams/${String(notification.data.team_id)}`));
		}
	}

	async function handleMarkAsRead(e: MouseEvent, id: string) {
		e.stopPropagation();
		await notifications.markAsRead(id);
	}

	async function handleDelete(e: MouseEvent, id: string) {
		e.stopPropagation();
		await notifications.deleteNotification(id);
	}

	function handleMarkAllAsRead() {
		notifications.markAllAsRead();
	}

	function handleClearRead() {
		notifications.clearRead();
	}
</script>

<svelte:head>
	<title>{m.notif_page_title()}</title>
</svelte:head>

<div class="mx-auto w-full max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
	<PageHeader
		title={m.notif_heading()}
		description={m.notif_summary({
			unread: notifState.summary.unread_count,
			total: notifState.summary.total_count
		})}
	>
		{#snippet actions()}
			{#if notifState.summary.unread_count > 0}
				<button type="button" onclick={handleMarkAllAsRead} class={BTN_SECONDARY}>
					<Icon icon="material-symbols:done-all" class="h-5 w-5 text-gray-400" />
					{m.notif_mark_all_read()}
				</button>
			{/if}
			{#if notifState.notifications.some((n) => n.read)}
				<button type="button" onclick={handleClearRead} class={BTN_GHOST}>
					{m.notif_clear_read()}
				</button>
			{/if}
		{/snippet}
	</PageHeader>

	<div class="mb-4">
		<SegmentedControl
			options={[
				{ id: 'all' as Filter, label: m.common_all() },
				{ id: 'unread' as Filter, label: m.notif_filter_unread() },
				{ id: 'read' as Filter, label: m.notif_filter_read() }
			]}
			value={filter}
			onChange={(id) => (filter = id)}
			size="sm"
		/>
	</div>

	<!-- Notification List -->
	<Panel flush>
		{#if notifState.loading && notifState.notifications.length === 0}
			<div class="p-5"><Skeleton class="h-14" rows={4} /></div>
		{:else if filteredNotifications.length === 0}
			<EmptyState
				dashed={false}
				icon="material-symbols:notifications-outline"
				title={m.notif_empty_heading()}
				description={filter === 'unread'
					? m.notif_empty_unread()
					: filter === 'read'
						? m.notif_empty_read()
						: m.notif_empty_all()}
			/>
		{:else}
			<div class="divide-y divide-gray-100 dark:divide-gray-700/60">
				{#each filteredNotifications as notification (notification.id)}
					{@const colors = getNotificationColors(notification)}
					<div
						role="button"
						tabindex="0"
						onclick={() => handleNotificationClick(notification)}
						onkeydown={(e) => e.key === 'Enter' && handleNotificationClick(notification)}
						class="group relative flex cursor-pointer items-start gap-4 px-5 py-4 transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40 {notification.read
							? 'opacity-60'
							: ''}"
					>
						{#if !notification.read}
							<div class="absolute bottom-0 left-0 top-0 w-0.5 bg-earthy-terracotta-500"></div>
						{/if}
						<div
							class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg {colors.bg}"
						>
							<Icon icon={getNotificationIcon(notification)} class="w-5 h-5 {colors.icon}" />
						</div>
						<div class="flex-1 min-w-0">
							<div class="flex items-start justify-between gap-4">
								<div>
									<p
										class="text-sm text-gray-900 dark:text-gray-100 {notification.read
											? ''
											: 'font-medium'}"
									>
										{renderNotificationTitle(notification)}
									</p>
									<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
										{renderNotificationMessage(notification)}
									</p>
									<div class="flex items-center gap-2 mt-2">
										<span class="text-xs text-gray-500 dark:text-gray-400">
											{formatRelativeTime(notification.created_at)}
										</span>
										{#if notification.recipient_type === 'team'}
											<Pill>{m.notif_via_team()}</Pill>
										{/if}
									</div>
								</div>
								<div class="flex items-center gap-1 flex-shrink-0">
									{#if !notification.read}
										<button
											type="button"
											onclick={(e) => handleMarkAsRead(e, notification.id)}
											class="rounded-md p-1.5 text-gray-400 opacity-0 transition hover:bg-gray-100 hover:text-gray-900 focus:opacity-100 group-hover:opacity-100 dark:hover:bg-gray-700 dark:hover:text-gray-100"
											aria-label={m.notif_mark_read_title()}
											title={m.notif_mark_read_title()}
										>
											<Icon icon="material-symbols:check-circle-outline" class="w-4 h-4" />
										</button>
									{/if}
									<button
										type="button"
										onclick={(e) => handleDelete(e, notification.id)}
										class="rounded-md p-1.5 text-gray-400 opacity-0 transition hover:bg-red-50 hover:text-red-600 focus:opacity-100 group-hover:opacity-100 dark:hover:bg-red-950/40 dark:hover:text-red-400"
										aria-label={m.notif_delete_title()}
										title={m.notif_delete_title()}
									>
										<Icon icon="material-symbols:delete-outline" class="w-4 h-4" />
									</button>
								</div>
							</div>
						</div>
					</div>
				{/each}
			</div>

			{#if notifState.hasMore}
				<div class="border-t border-gray-100 p-3 dark:border-gray-700/60">
					<button
						type="button"
						onclick={() => notifications.loadMore()}
						disabled={notifState.loading}
						class="{BTN_GHOST} w-full"
					>
						{notifState.loading ? m.common_loading() : m.notif_load_more()}
					</button>
				</div>
			{/if}
		{/if}
	</Panel>
</div>
