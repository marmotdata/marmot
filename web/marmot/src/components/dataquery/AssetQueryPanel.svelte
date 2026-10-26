<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { listQueryHistory } from '$lib/dataquery/api';
	import { clientLabel, outcome, subjectParts } from '$lib/dataquery/format';
	import type { QueryRecord } from '$lib/dataquery/types';
	import QueryDetail from './QueryDetail.svelte';

	let { mrn, name }: { mrn: string; name: string } = $props();

	let queries = $state<QueryRecord[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');
	let selectedId = $state<string | null>(null);

	let callers = $derived(new Set(queries.map((q) => q.audit_subject)).size);
	let lastSource = $derived(queries.find((q) => q.decision === 'allowed')?.source_name ?? '');
	let consoleQuery = $derived(
		new URLSearchParams({
			...(lastSource ? { source: lastSource } : {}),
			q: `SELECT *\nFROM ${name}\nLIMIT 10`
		}).toString()
	);

	$effect(() => {
		const target = mrn;
		loading = true;
		error = '';
		listQueryHistory({ mrn: target, limit: 15 })
			.then((data) => {
				queries = data.queries;
				total = data.total;
			})
			.catch((e) => (error = getErrorMessage(e)))
			.finally(() => (loading = false));
	});
</script>

<div class="mt-6 space-y-5">
	<div
		class="flex flex-col gap-4 rounded-2xl border border-gray-200 bg-gradient-to-r from-earthy-brown-50 to-white p-5 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700 dark:from-gray-800 dark:to-gray-900"
	>
		<div class="flex items-center gap-4">
			<div
				class="flex h-11 w-11 items-center justify-center rounded-xl bg-gray-900 text-earthy-terracotta-300 shadow-inner"
			>
				<IconifyIcon icon="material-symbols:shield-lock-outline" class="h-6 w-6" />
			</div>
			<div>
				<div class="font-semibold text-gray-900 dark:text-gray-100">
					{m.dataquery_asset_heading()}
				</div>
				<div class="text-sm text-gray-600 dark:text-gray-400">
					{#if loading}
						&nbsp;
					{:else if total === 0}
						{m.dataquery_asset_never()}
					{:else}
						{m.dataquery_asset_summary({
							count: formatNumber(total),
							callers: formatNumber(callers),
							when: formatRelativeTime(queries[0].started_at)
						})}
					{/if}
				</div>
			</div>
		</div>
		<a
			href={resolve(`/query?${consoleQuery}`)}
			class="inline-flex items-center justify-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
		>
			<IconifyIcon icon="material-symbols:terminal" class="h-4 w-4" />
			{m.dataquery_asset_query_button()}
		</a>
	</div>

	{#if error}
		<p class="text-sm text-red-600 dark:text-red-400">{error}</p>
	{:else if !loading && queries.length > 0}
		<ul
			class="divide-y divide-gray-100 overflow-hidden rounded-xl border border-gray-200 bg-white dark:divide-gray-700 dark:border-gray-700 dark:bg-gray-800"
		>
			{#each queries as q (q.id)}
				{@const badge = outcome(q)}
				{@const who = subjectParts(q.audit_subject)}
				<li>
					<button
						type="button"
						onclick={() => (selectedId = q.id)}
						class="flex w-full items-center gap-4 px-4 py-3 text-left transition hover:bg-earthy-brown-50 dark:hover:bg-gray-700/50"
					>
						<IconifyIcon
							icon={q.principal_type === 'user'
								? 'material-symbols:person-outline'
								: 'material-symbols:smart-toy-outline'}
							class="h-5 w-5 flex-shrink-0 text-gray-400"
						/>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2 text-sm">
								<span class="truncate font-medium text-gray-900 dark:text-gray-100">{who.name}</span
								>
								<span class="text-xs text-gray-400">{clientLabel(q.client)}</span>
							</div>
							<code class="block truncate font-mono text-xs text-gray-500 dark:text-gray-400"
								>{q.statement}</code
							>
						</div>
						<span class="rounded-full px-2 py-0.5 text-xs font-medium {badge.classes}"
							>{badge.label}</span
						>
						<span class="hidden w-24 text-right text-xs text-gray-500 sm:block dark:text-gray-400"
							>{formatRelativeTime(q.started_at)}</span
						>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

{#if selectedId}
	<QueryDetail queryId={selectedId} onClose={() => (selectedId = null)} />
{/if}
