<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { outcome } from '$lib/dataquery/format';
	import type { QueryRecord } from '$lib/dataquery/types';

	// A condensed view of the signed-in user's recent queries for the workspace's History tab; the
	// full, filterable log lives in the History drawer, opened via onViewAll.
	let {
		recent,
		onOpen,
		onViewAll
	}: {
		recent: QueryRecord[];
		onOpen: (query: QueryRecord, andRun?: boolean) => void;
		onViewAll: () => void;
	} = $props();

	function oneLine(text: string): string {
		return text.replace(/\s+/g, ' ').trim();
	}
</script>

{#if recent.length === 0}
	<p class="p-6 text-sm text-gray-500 dark:text-gray-400">
		{m.dataquery_workspace_history_empty()}
	</p>
{:else}
	<ul class="divide-y divide-gray-100 dark:divide-gray-800">
		{#each recent as query (query.id)}
			{@const o = outcome(query)}
			<li class="group flex items-center gap-3 px-4 py-2.5">
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium {o.classes}">{o.label}</span>
				<button
					type="button"
					onclick={() => onOpen(query)}
					class="min-w-0 flex-1 text-left"
					title={m.dataquery_workspace_open()}
				>
					<span
						class="block truncate font-mono text-xs text-gray-800 group-hover:text-earthy-terracotta-700 dark:text-gray-200 dark:group-hover:text-earthy-terracotta-400"
						>{oneLine(query.statement)}</span
					>
					<span class="text-[11px] text-gray-400"
						>{query.source_name} · {formatRelativeTime(query.started_at)}{query.rows_returned !=
						null
							? ` · ${m.dataquery_console_rows({ count: formatNumber(query.rows_returned) })}`
							: ''}</span
					>
				</button>
				<button
					type="button"
					onclick={() => onOpen(query, true)}
					class="rounded-md p-1.5 text-gray-400 opacity-0 transition hover:bg-gray-100 hover:text-earthy-terracotta-700 focus:opacity-100 group-hover:opacity-100 dark:hover:bg-gray-800"
					title={m.dataquery_workspace_rerun()}
					aria-label={m.dataquery_workspace_rerun()}
				>
					<IconifyIcon icon="material-symbols:replay" class="h-4 w-4" />
				</button>
			</li>
		{/each}
	</ul>
	<button
		type="button"
		onclick={onViewAll}
		class="m-3 text-xs font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
		>{m.dataquery_workspace_history_all()}</button
	>
{/if}
