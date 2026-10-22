<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
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

<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
	<div class="flex items-start justify-between gap-4 mb-2">
		<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
			<IconifyIcon
				icon="material-symbols:memory-outline"
				class="h-5 w-5 mr-2 text-earthy-terracotta-600"
			/>
			{m.dataquery_status_heading()}
		</h2>
		<button
			type="button"
			onclick={load}
			disabled={loading}
			class="p-1.5 rounded-md text-gray-500 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-40"
			title={m.dataquery_status_refresh()}
			aria-label={m.dataquery_status_refresh()}
		>
			<IconifyIcon
				icon="material-symbols:refresh"
				class="h-5 w-5 {loading ? 'animate-spin' : ''}"
			/>
		</button>
	</div>
	<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">{m.dataquery_status_description()}</p>

	{#if loading && instances.length === 0}
		<div class="flex items-center justify-center py-6">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
	{:else if instances.length === 0}
		<p class="text-sm text-gray-500 dark:text-gray-400">{m.dataquery_status_empty()}</p>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr
						class="text-left text-[11px] uppercase tracking-wider text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
					>
						<th class="py-2 pr-4 font-semibold">{m.dataquery_status_col_plugin()}</th>
						<th class="py-2 pr-4 font-semibold">{m.dataquery_status_col_started()}</th>
						<th class="py-2 pr-4 font-semibold">{m.dataquery_status_col_last_used()}</th>
						<th class="py-2 pr-4 font-semibold text-right">{m.dataquery_status_col_active()}</th>
						<th class="py-2 font-semibold text-right">{m.dataquery_status_col_restarts()}</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-700/60">
					{#each instances as instance (instance.plugin_id)}
						<tr>
							<td class="py-2.5 pr-4">
								<div class="flex items-center gap-2">
									<Icon name={instance.plugin_id} size="xs" showLabel={false} />
									<span class="text-gray-900 dark:text-gray-100 capitalize"
										>{instance.plugin_id}</span
									>
								</div>
							</td>
							<td class="py-2.5 pr-4 text-gray-600 dark:text-gray-400"
								>{formatRelativeTime(instance.started_at)}</td
							>
							<td class="py-2.5 pr-4 text-gray-600 dark:text-gray-400"
								>{formatRelativeTime(instance.last_used_at)}</td
							>
							<td class="py-2.5 pr-4 text-right tabular-nums text-gray-900 dark:text-gray-100"
								>{formatNumber(instance.active_queries)}</td
							>
							<td
								class="py-2.5 text-right tabular-nums {instance.restarts > 0
									? 'text-amber-700 dark:text-amber-300'
									: 'text-gray-900 dark:text-gray-100'}">{formatNumber(instance.restarts)}</td
							>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
