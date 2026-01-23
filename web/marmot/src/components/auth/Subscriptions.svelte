<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import IconifyIcon from '@iconify/svelte';

	interface SubscriptionWithAsset {
		id: string;
		asset_id: string;
		user_id: string;
		notification_types: string[];
		asset_name: string;
		asset_mrn: string;
		asset_type: string;
		created_at: string;
	}

	const typeLabels: Record<string, string> = {
		asset_change: 'Asset Changes',
		schema_change: 'Schema Changes',
		upstream_schema_change: 'Upstream Schema',
		downstream_schema_change: 'Downstream Schema',
		lineage_change: 'Lineage Changes'
	};

	let subscriptions: SubscriptionWithAsset[] = $state([]);
	let loading = $state(true);

	onMount(async () => {
		await fetchSubscriptions();
	});

	async function fetchSubscriptions() {
		try {
			const response = await fetchApi('/subscriptions/list');
			if (response.ok) {
				const data = await response.json();
				subscriptions = data.subscriptions || [];
			}
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	}

	async function unsubscribe(sub: SubscriptionWithAsset) {
		try {
			const response = await fetchApi(`/subscriptions/${sub.id}`, {
				method: 'DELETE'
			});
			if (response.ok) {
				subscriptions = subscriptions.filter((s) => s.id !== sub.id);
			}
		} catch {
			// ignore
		}
	}

	function getAssetLink(mrn: string): string {
		if (!mrn) return '#';
		const path = mrn.replace('mrn://', '');
		return `/discover/${path}`;
	}
</script>

<div>
	<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Subscriptions</h2>
	<p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
		Assets you're subscribed to for notifications. Manage notification types per asset or
		unsubscribe.
	</p>

	{#if loading}
		<div class="flex items-center justify-center py-8">
			<div class="animate-spin h-6 w-6 border-b-2 border-earthy-terracotta-700 rounded-full"></div>
		</div>
	{:else if subscriptions.length === 0}
		<div class="text-center py-8">
			<IconifyIcon
				icon="material-symbols:notifications-off-outline"
				class="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto mb-3"
			/>
			<p class="text-sm text-gray-500 dark:text-gray-400">
				No active subscriptions. Subscribe to assets from their detail page.
			</p>
		</div>
	{:else}
		<div class="space-y-3">
			{#each subscriptions as sub (sub.id)}
				<div
					class="flex items-center justify-between p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg"
				>
					<div class="flex items-center gap-3 min-w-0">
						<div class="min-w-0">
							<a
								href={getAssetLink(sub.asset_mrn)}
								class="text-sm font-medium text-gray-900 dark:text-gray-100 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 truncate block"
							>
								{sub.asset_name || 'Unknown Asset'}
							</a>
							<div class="flex items-center gap-2 mt-1">
								{#if sub.asset_type}
									<span
										class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
									>
										{sub.asset_type}
									</span>
								{/if}
								<div class="flex flex-wrap gap-1">
									{#each sub.notification_types as type}
										<span
											class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
										>
											{typeLabels[type] || type}
										</span>
									{/each}
								</div>
							</div>
						</div>
					</div>
					<button
						type="button"
						onclick={() => unsubscribe(sub)}
						class="flex-shrink-0 ml-3 p-1.5 text-gray-400 hover:text-red-500 dark:hover:text-red-400 rounded transition-colors cursor-pointer"
						title="Unsubscribe"
					>
						<IconifyIcon icon="material-symbols:notifications-off-outline" class="w-4 h-4" />
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>
