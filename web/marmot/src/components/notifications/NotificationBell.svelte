<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Icon from '@iconify/svelte';
	import { notifications, unreadCount } from '$lib/stores/notifications';
	import { auth } from '$lib/stores/auth';

	onMount(() => {
		if ($auth) {
			notifications.startPolling();
		}
	});

	onDestroy(() => {
		notifications.stopPolling();
	});
</script>

<a
	href="/notifications"
	class="relative p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors rounded-full hover:bg-gray-100 dark:hover:bg-gray-800"
	aria-label="Notifications"
>
	<Icon icon="material-symbols:notifications-outline" class="w-5 h-5" />
	{#if $unreadCount > 0}
		<span
			class="absolute top-0 right-0 flex items-center justify-center min-w-[18px] h-[18px] px-1 text-xs font-bold text-white bg-red-500 rounded-full transform translate-x-1 -translate-y-1"
		>
			{$unreadCount > 99 ? '99+' : $unreadCount}
		</span>
	{/if}
</a>
