<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { BTN_GHOST, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { getQueryStatus } from '$lib/dataquery/api';
	import type { QueryInstanceStatus } from '$lib/dataquery/types';

	let instances = $state<QueryInstanceStatus[]>([]);
	let loading = $state(true);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			instances = await getQueryStatus();
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<Panel
	flush
	title={m.dataquery_status_heading()}
	description={m.dataquery_status_description()}
	icon="material-symbols:memory-outline"
>
	{#snippet action()}
		<button
			type="button"
			onclick={load}
			disabled={loading}
			class={BTN_GHOST}
			title={m.dataquery_status_refresh()}
			aria-label={m.dataquery_status_refresh()}
		>
			<IconifyIcon
				icon="material-symbols:refresh"
				class="h-5 w-5 {loading ? 'animate-spin' : ''}"
			/>
		</button>
	{/snippet}

	{#if loading && instances.length === 0}
		<div class="p-5"><Skeleton class="h-10" rows={2} /></div>
	{:else if error}
		<div class="p-5"><Alert>{error}</Alert></div>
	{:else if instances.length === 0}
		<EmptyState
			dashed={false}
			icon="material-symbols:memory-outline"
			title={m.dataquery_status_empty()}
		/>
	{:else}
		<div class="overflow-x-auto">
			<table class="{TABLE} w-full">
				<thead>
					<tr class="border-b border-gray-100 dark:border-gray-700/60">
						<th class={TH}>{m.dataquery_status_col_plugin()}</th>
						<th class={TH}>{m.dataquery_status_col_started()}</th>
						<th class={TH}>{m.dataquery_status_col_last_used()}</th>
						<th class="{TH} text-right">{m.dataquery_status_col_active()}</th>
						<th class="{TH} text-right">{m.dataquery_status_col_restarts()}</th>
					</tr>
				</thead>
				<tbody class={TBODY}>
					{#each instances as instance (instance.plugin_id)}
						<tr class={TR}>
							<td class={TD}>
								<div class="flex items-center gap-2">
									<Icon name={instance.plugin_id} size="xs" showLabel={false} />
									<span class="font-medium capitalize text-gray-900 dark:text-gray-100"
										>{instance.plugin_id}</span
									>
								</div>
							</td>
							<td class="{TD} text-gray-500 dark:text-gray-400"
								>{formatRelativeTime(instance.started_at)}</td
							>
							<td class="{TD} text-gray-500 dark:text-gray-400"
								>{formatRelativeTime(instance.last_used_at)}</td
							>
							<td class="{TD} text-right tabular-nums">{formatNumber(instance.active_queries)}</td>
							<td class="{TD} text-right tabular-nums">
								{#if instance.restarts > 0}
									<Pill tone="warning">{formatNumber(instance.restarts)}</Pill>
								{:else}
									{formatNumber(instance.restarts)}
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Panel>
