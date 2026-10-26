<script lang="ts">
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import IconifyIcon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { searchAssetRules } from '$lib/assetrules/api';
	import type { AssetRule, AssetRulesListResponse } from '$lib/assetrules/types';
	import { m } from '$lib/paraglide/messages';
	import { formatDate, formatRelativeTime } from '$lib/utils';

	let rules = $state<AssetRule[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;

	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	type StatusFilter = 'all' | 'active' | 'disabled';
	let status = $state<StatusFilter>('all');
	let activeCount = $derived(rules.filter((r) => r.is_enabled).length);
	let shown = $derived(
		status === 'all'
			? rules
			: rules.filter((r) => (status === 'active' ? r.is_enabled : !r.is_enabled))
	);

	$effect(() => {
		if (browser) {
			loadRules();
		}
	});

	async function loadRules() {
		isLoading = true;
		error = null;
		try {
			const result: AssetRulesListResponse = await searchAssetRules(searchQuery, 0, 100);
			const allRules = result.asset_rules || [];

			// Sort by updated_at for recent rules (most recent first)
			const sortedByRecent = [...allRules].sort(
				(a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
			);

			rules = sortedByRecent;
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : m.assetrules_load_error();
		} finally {
			isLoading = false;
		}
	}

	function handleSearch(e: Event) {
		searchQuery = (e.target as HTMLInputElement).value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => loadRules(), 300);
	}

	function relativeTime(dateString: string | undefined): string {
		if (!dateString) return m.common_never();
		return formatRelativeTime(dateString);
	}
</script>

<svelte:head>
	<title>{m.assetrules_page_title()}</title>
</svelte:head>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
				{m.assetrules_heading()}
			</h1>
			<p class="mt-1 max-w-3xl text-sm text-gray-600 dark:text-gray-400">
				{m.assetrules_subheading()}
			</p>
		</div>
		{#if canManage}
			<a
				href={resolve('/asset-rules/new')}
				class="inline-flex flex-shrink-0 items-center gap-2 self-start rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500"
			>
				<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
				{m.assetrules_new_rule_button()}
			</a>
		{/if}
	</div>

	<div class="mt-6">
		{#if error}
			<div
				class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{error}
			</div>
		{:else if isLoading && rules.length === 0}
			<div class="space-y-2">
				{#each Array(5) as _, i (i)}
					<div class="h-14 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
				{/each}
			</div>
		{:else if rules.length === 0 && !searchQuery}
			<div class="mx-auto max-w-lg py-16 text-center">
				<span
					class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400"
				>
					<IconifyIcon icon="material-symbols:rule-settings" class="h-6 w-6" />
				</span>
				<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
					{m.assetrules_empty_heading()}
				</h2>
				<p class="mx-auto mt-2 max-w-md text-sm text-gray-600 dark:text-gray-400">
					{m.assetrules_empty_description()}
				</p>
				{#if canManage}
					<a
						href={resolve('/asset-rules/new')}
						class="mt-6 inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
					>
						<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
						{m.assetrules_create_first_button()}
					</a>
				{/if}
			</div>
		{:else}
			<div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
				<div class="flex flex-wrap items-center gap-2" role="group">
					{#each [{ id: 'all', label: m.assetrules_filter_all(), count: rules.length }, { id: 'active', label: m.common_active(), count: activeCount }, { id: 'disabled', label: m.common_disabled(), count: rules.length - activeCount }] as f (f.id)}
						<button
							type="button"
							onclick={() => (status = f.id as StatusFilter)}
							aria-pressed={status === f.id}
							class="inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium transition {status ===
							f.id
								? 'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900'
								: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200'}"
						>
							{f.label}
							<span
								class="rounded-full px-1.5 text-xs tabular-nums {status === f.id
									? 'bg-white/20'
									: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
								>{f.count}</span
							>
						</button>
					{/each}
				</div>
				<label
					class="flex w-full items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 py-2 focus-within:border-earthy-terracotta-500 focus-within:ring-2 focus-within:ring-earthy-terracotta-500/30 lg:w-80 dark:border-gray-600 dark:bg-gray-800"
				>
					<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
					<input
						value={searchQuery}
						oninput={handleSearch}
						placeholder={m.assetrules_search_placeholder()}
						aria-label={m.assetrules_search_placeholder()}
						class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
					/>
				</label>
			</div>

			<div
				class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
			>
				<div
					class="hidden items-center gap-4 border-b border-gray-200 bg-gray-50 px-4 py-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500 sm:flex dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-400"
					aria-hidden="true"
				>
					<span class="min-w-0 flex-1">{m.assetrules_col_rule()}</span>
					<span class="w-24 text-right">{m.assetrules_col_matches()}</span>
					<span class="hidden w-28 text-right md:block">{m.assetrules_col_applies()}</span>
					<span class="w-24">{m.common_status()}</span>
					<span class="w-24 text-right">{m.assetrules_col_updated()}</span>
				</div>
				{#if shown.length === 0}
					<p class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
						{m.assetrules_no_matching_hint()}
					</p>
				{/if}
				<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
					{#each shown as rule (rule.id)}
						<li>
							<a
								href={resolve('/asset-rules/[id]', { id: rule.id })}
								class="group flex items-center gap-4 px-4 py-3 transition hover:bg-gray-50 dark:hover:bg-gray-700/30"
							>
								<span class="flex min-w-0 flex-1 items-center gap-3">
									<span
										class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-earthy-terracotta-100 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900/30 dark:text-earthy-terracotta-300"
									>
										<IconifyIcon icon="material-symbols:rule-settings" class="h-5 w-5" />
									</span>
									<span class="min-w-0">
										<span
											class="block truncate font-medium text-gray-900 group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
											>{rule.name}</span
										>
										<span class="block truncate text-xs text-gray-500 dark:text-gray-400">
											{rule.description || m.assetrules_no_description()}
										</span>
									</span>
								</span>
								<span
									class="w-24 text-right text-sm tabular-nums text-gray-700 dark:text-gray-300"
									title={m.assetrules_matched_assets_title()}>{rule.membership_count || 0}</span
								>
								<span
									class="hidden w-28 items-center justify-end gap-3 text-xs text-gray-500 md:flex dark:text-gray-400"
								>
									<span class="inline-flex items-center gap-1" title={m.assetrules_links_title()}>
										<IconifyIcon icon="material-symbols:link" class="h-3.5 w-3.5" />
										{rule.links?.length ?? 0}
									</span>
									<span class="inline-flex items-center gap-1" title={m.assetrules_terms_title()}>
										<IconifyIcon icon="material-symbols:book-outline" class="h-3.5 w-3.5" />
										{rule.term_ids?.length ?? 0}
									</span>
								</span>
								<span class="w-24">
									<span
										class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium {rule.is_enabled
											? 'bg-earthy-green-100 text-earthy-green-900 dark:bg-earthy-green-900/30 dark:text-earthy-green-200'
											: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
									>
										<span
											class="h-1.5 w-1.5 rounded-full {rule.is_enabled
												? 'bg-earthy-green-500'
												: 'bg-gray-400'}"
										></span>
										{rule.is_enabled ? m.common_active() : m.common_disabled()}
									</span>
								</span>
								<span
									class="w-24 text-right text-xs text-gray-500 dark:text-gray-400"
									title={rule.updated_at ? formatDate(rule.updated_at) : ''}
									>{relativeTime(rule.updated_at)}</span
								>
							</a>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
</div>
