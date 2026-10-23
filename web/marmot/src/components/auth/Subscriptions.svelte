<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import IconifyIcon from '@iconify/svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { BTN_GHOST, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';

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
		asset_change: m.subscriptions_type_asset_change(),
		schema_change: m.subscriptions_type_schema_change(),
		upstream_schema_change: m.subscriptions_type_upstream_schema(),
		downstream_schema_change: m.subscriptions_type_downstream_schema(),
		lineage_change: m.subscriptions_type_lineage_change()
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
</script>

<Panel
	flush
	title={m.subscriptions_heading()}
	description={m.subscriptions_description()}
	icon="material-symbols:notifications-outline"
>
	{#if loading}
		<div class="p-5"><Skeleton class="h-12" rows={3} /></div>
	{:else if subscriptions.length === 0}
		<EmptyState
			dashed={false}
			icon="material-symbols:notifications-off-outline"
			title={m.subscriptions_empty()}
		/>
	{:else}
		<div class="overflow-x-auto">
			<table class="{TABLE} w-full">
				<thead>
					<tr class="border-b border-gray-100 dark:border-gray-700/60">
						<th class={TH}>{m.subscriptions_column_asset()}</th>
						<th class={TH}>{m.common_type()}</th>
						<th class={TH}>{m.subscriptions_column_notifications()}</th>
						<th class="{TH} text-right"><span class="sr-only">{m.common_actions()}</span></th>
					</tr>
				</thead>
				<tbody class={TBODY}>
					{#each subscriptions as sub (sub.id)}
						<tr class={TR}>
							<td class="{TD} max-w-xs">
								<a
									href={sub.asset_mrn
										? resolve(
												`/discover/${sub.asset_mrn.replace('mrn://', '')}` as `/discover/${string}/${string}/${string}`
											)
										: '#'}
									class="block truncate font-medium text-gray-900 hover:text-earthy-terracotta-700 dark:text-gray-100 dark:hover:text-earthy-terracotta-400"
								>
									{sub.asset_name || m.subscriptions_unknown_asset()}
								</a>
							</td>
							<td class="{TD} whitespace-nowrap">
								{#if sub.asset_type}<Pill>{sub.asset_type}</Pill>{/if}
							</td>
							<td class={TD}>
								<div class="flex flex-wrap gap-1">
									{#each sub.notification_types as type (type)}
										<Pill tone="brand">{typeLabels[type] || type}</Pill>
									{/each}
								</div>
							</td>
							<td class="{TD} text-right">
								<button
									type="button"
									onclick={() => unsubscribe(sub)}
									class="{BTN_GHOST} hover:!bg-red-50 hover:!text-red-700 dark:hover:!bg-red-950/40 dark:hover:!text-red-300"
									title={m.subscriptions_unsubscribe_title()}
									aria-label={m.subscriptions_unsubscribe_title()}
								>
									<IconifyIcon icon="material-symbols:notifications-off-outline" class="h-4 w-4" />
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Panel>
