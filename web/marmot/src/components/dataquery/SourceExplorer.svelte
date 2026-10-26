<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import Icon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatNumber } from '$lib/utils/format';
	import { listQuerySourceTables, listQuerySources } from '$lib/dataquery/api';
	import { assetRouteParams } from '$lib/dataquery/format';
	import type { QuerySource, QuerySourceTable } from '$lib/dataquery/types';

	let {
		active,
		onSelect,
		onInsert,
		onPreview
	}: {
		active: string;
		onSelect: (source: string) => void;
		onInsert: (text: string) => void;
		onPreview: (source: string, statement: string) => void;
	} = $props();

	const PAGE = 50;

	let search = $state('');
	let sources = $state<QuerySource[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');

	const expanded = new SvelteSet<string>();
	const tables = new SvelteMap<string, QuerySourceTable[]>();
	const tableFilters = new SvelteMap<string, string>();
	const tablesLoading = new SvelteSet<string>();

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let tableTimer: ReturnType<typeof setTimeout> | undefined;
	let sourcesRequest = 0;

	async function loadSources(append = false) {
		const id = ++sourcesRequest;
		if (append) loadingMore = true;
		else loading = true;
		error = '';
		try {
			const data = await listQuerySources({
				q: search.trim(),
				limit: PAGE,
				offset: append ? sources.length : 0
			});
			if (id !== sourcesRequest) return;
			sources = append ? [...sources, ...data.sources] : data.sources;
			total = data.total;
		} catch (e) {
			if (id === sourcesRequest) error = getErrorMessage(e);
		} finally {
			if (id === sourcesRequest) {
				loading = false;
				loadingMore = false;
			}
		}
	}

	async function loadTables(source: string) {
		tablesLoading.add(source);
		try {
			const filter = tableFilters.get(source)?.trim() ?? '';
			tables.set(source, await listQuerySourceTables(source, { q: filter, limit: 100 }));
		} catch {
			tables.set(source, []);
		} finally {
			tablesLoading.delete(source);
		}
	}

	function toggle(source: string) {
		if (expanded.has(source)) {
			expanded.delete(source);
			return;
		}
		expanded.add(source);
		if (!tables.has(source)) loadTables(source);
	}

	// Clicking a source's name selects and opens it; clicking the source you are already on folds it away or back out, like a file tree
	function choose(source: string) {
		if (source === active) {
			toggle(source);
			return;
		}
		onSelect(source);
		if (!expanded.has(source)) toggle(source);
	}

	function onSearch() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => loadSources(), 200);
	}

	function onTableFilter(source: string, value: string) {
		tableFilters.set(source, value);
		clearTimeout(tableTimer);
		tableTimer = setTimeout(() => loadTables(source), 200);
	}

	// A newly active source opens on its own so the tables you are querying are in view; it only reacts to the source changing, so collapsing it by hand sticks
	let opened = '';
	$effect(() => {
		if (!active || active === opened) return;
		opened = active;
		untrack(() => {
			if (!expanded.has(active)) {
				expanded.add(active);
				if (!tables.has(active)) loadTables(active);
			}
		});
	});

	onMount(() => loadSources());
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="flex items-center justify-between px-3 pb-2 pt-3">
		<span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
			>{m.dataquery_explorer_heading()}</span
		>
		{#if total > 0}
			<span class="text-xs tabular-nums text-gray-400">{formatNumber(total)}</span>
		{/if}
	</div>
	<div class="px-3 pb-2">
		<div class="relative">
			<IconifyIcon
				icon="material-symbols:search"
				class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
			/>
			<input
				type="search"
				bind:value={search}
				oninput={onSearch}
				placeholder={m.dataquery_explorer_search()}
				aria-label={m.dataquery_explorer_search()}
				class="w-full rounded-md border-gray-200 bg-white py-1.5 pl-8 pr-2 text-sm focus:border-earthy-terracotta-500 focus:ring-earthy-terracotta-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
			/>
		</div>
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto px-1.5 pb-3">
		{#if loading}
			<div class="space-y-1.5 px-1.5 pt-1">
				{#each Array(6) as _, i (i)}
					<div class="h-7 animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
				{/each}
			</div>
		{:else if error}
			<p class="px-2 pt-2 text-sm text-red-600 dark:text-red-400">{error}</p>
		{:else if sources.length === 0}
			<p class="px-2 pt-2 text-sm text-gray-500 dark:text-gray-400">
				{search ? m.dataquery_explorer_no_match() : m.dataquery_empty_title()}
			</p>
		{:else}
			<ul role="tree" aria-label={m.dataquery_explorer_heading()}>
				{#each sources as source (source.id)}
					{@const open = expanded.has(source.name)}
					{@const list = tables.get(source.name) ?? []}
					<li role="treeitem" aria-expanded={open} aria-selected={source.name === active}>
						<div
							class="group flex items-center rounded-md {source.name === active
								? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
								: 'hover:bg-gray-100 dark:hover:bg-gray-800'}"
						>
							<button
								type="button"
								onclick={() => toggle(source.name)}
								class="flex h-7 w-6 flex-shrink-0 items-center justify-center text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
								aria-label={open ? m.dataquery_explorer_collapse() : m.dataquery_explorer_expand()}
							>
								<IconifyIcon
									icon="material-symbols:chevron-right"
									class="h-4 w-4 transition-transform {open ? 'rotate-90' : ''}"
								/>
							</button>
							<button
								type="button"
								onclick={() => choose(source.name)}
								class="flex min-w-0 flex-1 items-center gap-2 py-1 pr-2 text-left text-sm"
							>
								<Icon name={source.plugin_id || 'database'} size="xs" showLabel={false} />
								<span
									class="truncate {source.name === active
										? 'font-semibold text-earthy-terracotta-800 dark:text-earthy-terracotta-200'
										: 'text-gray-800 dark:text-gray-200'}">{source.name}</span
								>
								<span class="ml-auto text-[11px] tabular-nums text-gray-400"
									>{formatNumber(source.table_count)}</span
								>
							</button>
						</div>

						{#if open}
							<div class="ml-3 border-l border-gray-100 pl-1.5 dark:border-gray-800">
								{#if (source.table_count ?? 0) > 12 || tableFilters.get(source.name)}
									<input
										type="search"
										value={tableFilters.get(source.name) ?? ''}
										oninput={(e) => onTableFilter(source.name, e.currentTarget.value)}
										placeholder={m.dataquery_explorer_filter_tables()}
										aria-label={m.dataquery_explorer_filter_tables()}
										class="my-1 ml-1.5 w-[calc(100%-0.75rem)] rounded border-gray-200 bg-white px-2 py-1 text-xs focus:border-earthy-terracotta-500 focus:ring-earthy-terracotta-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
									/>
								{/if}
								{#if tablesLoading.has(source.name) && list.length === 0}
									<div class="space-y-1 py-1 pl-1.5">
										{#each Array(3) as _, i (i)}
											<div class="h-6 animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
										{/each}
									</div>
								{:else if list.length === 0}
									<p class="py-1.5 pl-2 text-xs text-gray-400">
										{m.dataquery_explorer_no_tables()}
									</p>
								{:else}
									<ul role="group">
										{#each list as table (table.mrn)}
											{@const route = table.asset_id ? assetRouteParams(table.mrn) : null}
											<li
												role="treeitem"
												aria-selected="false"
												class="group/table flex items-center rounded-md hover:bg-gray-100 dark:hover:bg-gray-800"
												title={table.description || table.mrn}
											>
												<button
													type="button"
													onclick={() =>
														table.sample_statement
															? onPreview(source.name, table.sample_statement)
															: onInsert(table.name)}
													class="flex min-w-0 flex-1 items-center gap-2 py-1 pl-1.5 pr-1 text-left text-[13px]"
													title={table.sample_statement
														? m.dataquery_explorer_preview()
														: m.dataquery_explorer_insert({ name: table.name })}
												>
													<IconifyIcon
														icon={table.type === 'View'
															? 'material-symbols:visibility-outline'
															: 'material-symbols:table-outline'}
														class="h-3.5 w-3.5 flex-shrink-0 text-gray-400"
													/>
													<span class="truncate font-mono text-gray-700 dark:text-gray-300"
														>{table.name}</span
													>
												</button>
												<div
													class="flex flex-shrink-0 items-center pr-1 opacity-0 transition-opacity focus-within:opacity-100 group-hover/table:opacity-100"
												>
													<button
														type="button"
														onclick={() => onInsert(table.name)}
														class="rounded p-1 text-gray-400 hover:bg-white hover:text-earthy-terracotta-700 dark:hover:bg-gray-700 dark:hover:text-earthy-terracotta-400"
														title={m.dataquery_explorer_insert({ name: table.name })}
														aria-label={m.dataquery_explorer_insert({ name: table.name })}
													>
														<IconifyIcon icon="material-symbols:input-rounded" class="h-4 w-4" />
													</button>
													{#if route}
														<a
															href={resolve('/discover/[type]/[service]/[name]', route)}
															class="rounded p-1 text-gray-400 hover:bg-white hover:text-earthy-terracotta-700 dark:hover:bg-gray-700 dark:hover:text-earthy-terracotta-400"
															title={m.dataquery_explorer_open_catalog()}
															aria-label={m.dataquery_explorer_open_catalog()}
														>
															<IconifyIcon
																icon="material-symbols:open-in-new"
																class="h-3.5 w-3.5"
															/>
														</a>
													{/if}
												</div>
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						{/if}
					</li>
				{/each}
			</ul>
			{#if sources.length < total}
				<button
					type="button"
					onclick={() => loadSources(true)}
					disabled={loadingMore}
					class="mt-2 w-full rounded-md py-1.5 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:hover:bg-gray-800 dark:hover:text-gray-200"
				>
					{m.dataquery_explorer_more({ count: formatNumber(total - sources.length) })}
				</button>
			{/if}
		{/if}
	</div>
</div>
