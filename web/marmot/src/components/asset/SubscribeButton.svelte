<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';

	interface Props {
		assetId: string;
	}

	let { assetId }: Props = $props();

	interface Subscription {
		id: string;
		asset_id: string;
		user_id: string;
		notification_types: string[];
	}

	const notificationTypes = [
		{ type: 'asset_change', label: 'Asset Changes' },
		{ type: 'schema_change', label: 'Schema Changes' },
		{ type: 'upstream_schema_change', label: 'Upstream Schema' },
		{ type: 'downstream_schema_change', label: 'Downstream Schema' },
		{ type: 'lineage_change', label: 'Lineage Changes' },
		{ type: 'asset_deleted', label: 'Asset Deletions' }
	];

	let subscription: Subscription | null = $state(null);
	let loading = $state(true);
	let showDropdown = $state(false);

	let isLoggedIn = $derived(!!auth.getToken());

	onMount(() => {
		if (isLoggedIn) {
			fetchSubscription();
		} else {
			loading = false;
		}
	});

	async function fetchSubscription() {
		try {
			const response = await fetchApi(`/subscriptions?asset_id=${assetId}`);
			if (response.ok) {
				const data = await response.json();
				subscription = data;
			}
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	}

	async function subscribe() {
		loading = true;
		try {
			const response = await fetchApi('/subscriptions', {
				method: 'POST',
				body: JSON.stringify({
					asset_id: assetId,
					notification_types: ['asset_change', 'schema_change']
				})
			});
			if (response.ok) {
				subscription = await response.json();
				showDropdown = true;
			}
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	}

	async function unsubscribe() {
		if (!subscription) return;
		loading = true;
		try {
			const response = await fetchApi(`/subscriptions/${subscription.id}`, {
				method: 'DELETE'
			});
			if (response.ok) {
				subscription = null;
				showDropdown = false;
			}
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	}

	async function toggleType(type: string) {
		if (!subscription) return;
		const current = subscription.notification_types;
		let updated: string[];
		if (current.includes(type)) {
			updated = current.filter((t) => t !== type);
			if (updated.length === 0) return; // Must keep at least one
		} else {
			updated = [...current, type];
		}

		try {
			const response = await fetchApi(`/subscriptions/${subscription.id}`, {
				method: 'PUT',
				body: JSON.stringify({ notification_types: updated })
			});
			if (response.ok) {
				subscription = await response.json();
			}
		} catch {
			// ignore
		}
	}

	function handleButtonClick() {
		if (subscription) {
			showDropdown = !showDropdown;
		} else {
			subscribe();
		}
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.subscribe-dropdown-container')) {
			showDropdown = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="subscribe-dropdown-container relative">
	<button
		type="button"
		onclick={handleButtonClick}
		disabled={loading || !isLoggedIn}
		class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-colors
			{!isLoggedIn
			? 'bg-gray-100 text-gray-400 cursor-not-allowed dark:bg-gray-700 dark:text-gray-500'
			: subscription
				? 'bg-earthy-terracotta-50 text-earthy-terracotta-700 hover:bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 dark:text-earthy-terracotta-400 dark:hover:bg-earthy-terracotta-900/30'
				: 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'}
			{loading ? 'opacity-50 cursor-not-allowed' : !isLoggedIn ? '' : 'cursor-pointer'}"
		title={!isLoggedIn
			? 'You must be logged in to subscribe to assets'
			: subscription
				? 'Manage subscription'
				: 'Subscribe to notifications'}
	>
		<IconifyIcon
			icon={subscription
				? 'material-symbols:notifications'
				: 'material-symbols:notifications-outline'}
			class="w-3.5 h-3.5"
		/>
		{subscription ? 'Subscribed' : 'Subscribe'}
	</button>

	{#if showDropdown && subscription}
		<div
			class="absolute top-full left-0 mt-1 w-56 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg z-50"
		>
			<div class="p-2 space-y-0.5">
				{#each notificationTypes as { type, label }}
					<button
						type="button"
						onclick={() => toggleType(type)}
						class="flex items-center gap-2 w-full px-2 py-1.5 text-xs rounded hover:bg-gray-50 dark:hover:bg-gray-700/50 text-left cursor-pointer"
					>
						<div
							class="w-3.5 h-3.5 rounded border flex items-center justify-center
								{subscription.notification_types.includes(type)
								? 'bg-earthy-terracotta-600 border-earthy-terracotta-600'
								: 'border-gray-300 dark:border-gray-600'}"
						>
							{#if subscription.notification_types.includes(type)}
								<IconifyIcon icon="material-symbols:check" class="w-2.5 h-2.5 text-white" />
							{/if}
						</div>
						<span class="text-gray-700 dark:text-gray-300">{label}</span>
					</button>
				{/each}
			</div>
			<div class="border-t border-gray-200 dark:border-gray-700 p-2">
				<button
					type="button"
					onclick={unsubscribe}
					class="w-full px-2 py-1.5 text-xs text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded text-left cursor-pointer"
				>
					Unsubscribe
				</button>
			</div>
		</div>
	{/if}
</div>
