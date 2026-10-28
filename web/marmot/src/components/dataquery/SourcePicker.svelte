<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import { listQuerySources } from '$lib/dataquery/api';
	import type { QuerySource } from '$lib/dataquery/types';

	let {
		value = $bindable(''),
		emptyLabel = '',
		onchange,
		size = 'md'
	}: {
		value?: string;
		emptyLabel?: string;
		onchange?: (name: string) => void;
		size?: 'sm' | 'md';
	} = $props();

	const PAGE = 20;
	const listId = `source-picker-${Math.random().toString(36).slice(2, 8)}`;

	let open = $state(false);
	let search = $state('');
	let results = $state<QuerySource[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let active = $state(0);
	let selectedPlugin = $state('');
	let pluginFor = '';
	let input = $state<HTMLInputElement | null>(null);
	let root = $state<HTMLDivElement | null>(null);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let requestId = 0;

	async function fetchResults(term: string) {
		const id = ++requestId;
		loading = true;
		try {
			const data = await listQuerySources({ q: term, limit: PAGE });
			if (id !== requestId) return;
			results = data.sources;
			total = data.total;
			active = 0;
		} catch {
			if (id === requestId) results = [];
		} finally {
			if (id === requestId) loading = false;
		}
	}

	function openList() {
		if (open) return;
		open = true;
		search = '';
		fetchResults('');
		queueMicrotask(() => input?.focus());
	}

	function close() {
		open = false;
		clearTimeout(timer);
	}

	function onSearch() {
		clearTimeout(timer);
		timer = setTimeout(() => fetchResults(search), 200);
	}

	function choose(source: QuerySource | null) {
		value = source?.name ?? '';
		selectedPlugin = source?.plugin_id ?? '';
		pluginFor = value;
		close();
		onchange?.(value);
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = Math.min(active + 1, results.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = Math.max(active - 1, 0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (results[active]) choose(results[active]);
		} else if (e.key === 'Escape') {
			close();
		}
	}

	function onFocusOut(e: FocusEvent) {
		if (root && !root.contains(e.relatedTarget as Node | null)) close();
	}

	// A value set from outside (a default or a link) arrives without its
	// plugin, so look it up to show the right icon.
	$effect(() => {
		const name = value;
		if (!name || pluginFor === name) return;
		pluginFor = name;
		selectedPlugin = '';
		listQuerySources({ q: name, limit: 10 })
			.then((data) => {
				if (value === name) {
					selectedPlugin = data.sources.find((s) => s.name === name)?.plugin_id ?? '';
				}
			})
			.catch(() => {});
	});

	const triggerSize = $derived(size === 'sm' ? 'py-2 text-sm' : 'py-2 text-sm min-w-[16rem]');
</script>

<div class="relative" bind:this={root} onfocusout={onFocusOut}>
	{#if open}
		<div
			class="flex items-center gap-2 rounded-lg border border-earthy-terracotta-500 bg-white px-3 ring-2 ring-earthy-terracotta-500/30 dark:bg-gray-800 {triggerSize}"
		>
			<IconifyIcon icon="material-symbols:search" class="h-4 w-4 flex-shrink-0 text-gray-400" />
			<input
				bind:this={input}
				bind:value={search}
				oninput={onSearch}
				onkeydown={onKeydown}
				role="combobox"
				aria-expanded="true"
				aria-controls={listId}
				aria-label={m.dataquery_source_picker_search()}
				placeholder={m.dataquery_source_picker_search()}
				class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
			/>
			{#if loading}
				<IconifyIcon
					icon="material-symbols:progress-activity"
					class="h-4 w-4 flex-shrink-0 animate-spin text-gray-400"
				/>
			{/if}
		</div>
		<div
			class="absolute left-0 z-30 mt-1 w-full min-w-[18rem] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
		>
			<ul id={listId} role="listbox" class="max-h-72 overflow-y-auto py-1">
				{#if emptyLabel}
					<li role="option" aria-selected={value === ''}>
						<button
							type="button"
							onclick={() => choose(null)}
							class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50"
						>
							<IconifyIcon icon="material-symbols:select-all" class="h-4 w-4 text-gray-400" />
							{emptyLabel}
						</button>
					</li>
				{/if}
				{#each results as source, i (source.id)}
					<li role="option" aria-selected={value === source.name}>
						<button
							type="button"
							onclick={() => choose(source)}
							onmouseenter={() => (active = i)}
							class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm {i === active
								? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
								: ''}"
						>
							<Icon name={source.plugin_id} size="xs" showLabel={false} />
							<span class="min-w-0 flex-1 truncate text-gray-900 dark:text-gray-100"
								>{source.name}</span
							>
							{#if value === source.name}
								<IconifyIcon
									icon="material-symbols:check"
									class="h-4 w-4 text-earthy-terracotta-600"
								/>
							{/if}
						</button>
					</li>
				{:else}
					{#if !loading}
						<li class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
							{m.dataquery_source_picker_none()}
						</li>
					{/if}
				{/each}
			</ul>
			{#if total > results.length}
				<div
					class="border-t border-gray-100 px-3 py-2 text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400"
				>
					{m.dataquery_source_picker_more({
						shown: formatNumber(results.length),
						total: formatNumber(total)
					})}
				</div>
			{/if}
		</div>
	{:else}
		<button
			type="button"
			onclick={openList}
			aria-haspopup="listbox"
			class="flex w-full items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-left transition hover:border-gray-400 dark:border-gray-600 dark:bg-gray-800 dark:hover:border-gray-500 {triggerSize}"
		>
			{#if value}
				{#if selectedPlugin}
					<Icon name={selectedPlugin} size="xs" showLabel={false} />
				{:else}
					<IconifyIcon icon="material-symbols:database-outline" class="h-4 w-4 text-gray-400" />
				{/if}
				<span class="min-w-0 flex-1 truncate text-gray-900 dark:text-gray-100">{value}</span>
			{:else}
				<IconifyIcon icon="material-symbols:database-outline" class="h-4 w-4 text-gray-400" />
				<span class="min-w-0 flex-1 truncate text-gray-500 dark:text-gray-400"
					>{emptyLabel || m.dataquery_source_picker_choose()}</span
				>
			{/if}
			<IconifyIcon
				icon="material-symbols:unfold-more"
				class="h-4 w-4 flex-shrink-0 text-gray-400"
			/>
		</button>
	{/if}
</div>
