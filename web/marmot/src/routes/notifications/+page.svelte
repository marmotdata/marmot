<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import { notifications, type Notification } from '$lib/stores/notifications';
	import { formatRelativeTime } from '$lib/utils/format';
	import { goto } from '$app/navigation';

	let filter: 'all' | 'unread' | 'read' = 'all';

	$: state = $notifications;
	$: filteredNotifications = state.notifications.filter((n) => {
		if (filter === 'unread') return !n.read;
		if (filter === 'read') return n.read;
		return true;
	});

	onMount(() => {
		notifications.refresh();
	});

	function getNotificationIcon(notification: Notification): string {
		switch (notification.type) {
			case 'asset_change':
				return 'material-symbols:database';
			case 'asset_deleted':
				return 'material-symbols:delete';
			case 'team_invite':
				return 'material-symbols:group-add';
			case 'mention':
				return 'material-symbols:alternate-email';
			case 'job_complete':
				if (notification.data?.status === 'failed') return 'material-symbols:error';
				if (notification.data?.status === 'cancelled') return 'material-symbols:cancel';
				return 'material-symbols:check-circle';
			case 'system':
			default:
				return 'material-symbols:info';
		}
	}

	function getNotificationColors(notification: Notification): { bg: string; icon: string } {
		switch (notification.type) {
			case 'asset_change':
				return {
					bg: 'bg-earthy-blue-100 dark:bg-earthy-blue-900/30',
					icon: 'text-earthy-blue-700 dark:text-earthy-blue-400'
				};
			case 'asset_deleted':
				return {
					bg: 'bg-red-100 dark:bg-red-900/30',
					icon: 'text-red-700 dark:text-red-400'
				};
			case 'team_invite':
				return {
					bg: 'bg-purple-100 dark:bg-purple-900/30',
					icon: 'text-purple-700 dark:text-purple-400'
				};
			case 'mention':
				return {
					bg: 'bg-earthy-green-100 dark:bg-earthy-green-900/30',
					icon: 'text-earthy-green-700 dark:text-earthy-green-400'
				};
			case 'job_complete':
				if (notification.data?.status === 'failed') {
					return {
						bg: 'bg-red-100 dark:bg-red-900/30',
						icon: 'text-red-700 dark:text-red-400'
					};
				}
				if (notification.data?.status === 'cancelled') {
					return {
						bg: 'bg-amber-100 dark:bg-amber-900/30',
						icon: 'text-amber-700 dark:text-amber-400'
					};
				}
				return {
					bg: 'bg-green-100 dark:bg-green-900/30',
					icon: 'text-green-700 dark:text-green-400'
				};
			case 'system':
			default:
				return {
					bg: 'bg-gray-100 dark:bg-gray-700',
					icon: 'text-gray-600 dark:text-gray-400'
				};
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
			goto(notification.data.link as string);
		} else if (notification.data?.asset_mrn) {
			goto(`/discover/${encodeURIComponent(notification.data.asset_mrn as string)}`);
		} else if (notification.data?.team_id) {
			goto(`/teams/${notification.data.team_id}`);
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
	<title>Notifications - Marmot</title>
</svelte:head>

<div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
	<!-- Header -->
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Notifications</h1>
			<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
				{state.summary.unread_count} unread, {state.summary.total_count} total
			</p>
		</div>
		<div class="flex items-center gap-2">
			{#if state.summary.unread_count > 0}
				<button
					type="button"
					onclick={handleMarkAllAsRead}
					class="px-3 py-1.5 text-sm font-medium text-earthy-terracotta-700 hover:bg-earthy-brown-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					Mark all as read
				</button>
			{/if}
			{#if state.notifications.some((n) => n.read)}
				<button
					type="button"
					onclick={handleClearRead}
					class="px-3 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
				>
					Clear read
				</button>
			{/if}
		</div>
	</div>

	<!-- Filter Tabs -->
	<div class="flex items-center gap-1 mb-6 border-b border-gray-200 dark:border-gray-700">
		<button
			type="button"
			onclick={() => (filter = 'all')}
			class="px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px {filter === 'all'
				? 'text-earthy-terracotta-700 border-earthy-terracotta-700'
				: 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-700 dark:hover:text-gray-300'}"
		>
			All
		</button>
		<button
			type="button"
			onclick={() => (filter = 'unread')}
			class="px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px flex items-center gap-1.5 {filter ===
			'unread'
				? 'text-earthy-terracotta-700 border-earthy-terracotta-700'
				: 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-700 dark:hover:text-gray-300'}"
		>
			Unread
			{#if state.summary.unread_count > 0}
				<span
					class="px-1.5 py-0.5 text-xs rounded-full bg-earthy-terracotta-700/10 text-earthy-terracotta-700"
				>
					{state.summary.unread_count}
				</span>
			{/if}
		</button>
		<button
			type="button"
			onclick={() => (filter = 'read')}
			class="px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px {filter === 'read'
				? 'text-earthy-terracotta-700 border-earthy-terracotta-700'
				: 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-700 dark:hover:text-gray-300'}"
		>
			Read
		</button>
	</div>

	<!-- Notification List -->
	<div
		class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
	>
		{#if state.loading && state.notifications.length === 0}
			<div class="flex items-center justify-center py-16">
				<div
					class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if filteredNotifications.length === 0}
			<div class="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
				<svg class="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="1.5"
						d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
					/>
				</svg>
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-1">No notifications</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					{#if filter === 'unread'}
						You're all caught up!
					{:else if filter === 'read'}
						No read notifications yet.
					{:else}
						You don't have any notifications yet.
					{/if}
				</p>
			</div>
		{:else}
			<div class="divide-y divide-gray-200 dark:divide-gray-700">
				{#each filteredNotifications as notification (notification.id)}
					{@const colors = getNotificationColors(notification)}
					<div
						role="button"
						tabindex="0"
						onclick={() => handleNotificationClick(notification)}
						onkeydown={(e) => e.key === 'Enter' && handleNotificationClick(notification)}
						class="relative flex items-start gap-4 p-4 transition-colors group cursor-pointer {notification.read
							? 'opacity-60 hover:bg-gray-50 dark:hover:bg-gray-700/50'
							: 'hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
					>
						{#if !notification.read}
							<div class="absolute left-0 top-0 bottom-0 w-0.5 bg-earthy-terracotta-700"></div>
						{/if}
						<div
							class="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center {colors.bg}"
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
										{notification.title}
									</p>
									<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
										{notification.message}
									</p>
									<div class="flex items-center gap-2 mt-2">
										<span class="text-xs text-gray-400 dark:text-gray-500">
											{formatRelativeTime(notification.created_at)}
										</span>
										{#if notification.recipient_type === 'team'}
											<span
												class="text-xs px-1.5 py-0.5 rounded-full bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300"
											>
												via team
											</span>
										{/if}
									</div>
								</div>
								<div class="flex items-center gap-1 flex-shrink-0">
									{#if !notification.read}
										<button
											type="button"
											onclick={(e) => handleMarkAsRead(e, notification.id)}
											class="p-1.5 text-gray-400 hover:text-earthy-green-700 dark:hover:text-earthy-green-400 opacity-0 group-hover:opacity-100 transition-all rounded-md hover:bg-gray-100 dark:hover:bg-gray-700"
											title="Mark as read"
										>
											<Icon icon="material-symbols:check-circle-outline" class="w-4 h-4" />
										</button>
									{/if}
									<button
										type="button"
										onclick={(e) => handleDelete(e, notification.id)}
										class="p-1.5 text-gray-400 hover:text-red-500 dark:hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all rounded-md hover:bg-gray-100 dark:hover:bg-gray-700"
										title="Delete notification"
									>
										<Icon icon="material-symbols:delete-outline" class="w-4 h-4" />
									</button>
								</div>
							</div>
						</div>
					</div>
				{/each}
			</div>

			{#if state.hasMore}
				<div class="p-4 border-t border-gray-200 dark:border-gray-700">
					<button
						type="button"
						onclick={() => notifications.loadMore()}
						disabled={state.loading}
						class="w-full py-2 text-sm font-medium text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors disabled:opacity-50 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
					>
						{state.loading ? 'Loading...' : 'Load more notifications'}
					</button>
				</div>
			{/if}
		{/if}
	</div>
</div>
