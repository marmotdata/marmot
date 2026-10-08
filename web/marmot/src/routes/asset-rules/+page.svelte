<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import { auth } from '$lib/stores/auth';
	import { searchAssetRules } from '$lib/assetrules/api';
	import type { AssetRule, AssetRulesListResponse } from '$lib/assetrules/types';
	import { m } from '$lib/paraglide/messages';
	import { formatDate, formatRelativeTime } from '$lib/utils';

	let rules = $state<AssetRule[]>([]);
	let recentRules = $state<AssetRule[]>([]);
	let total = $state(0);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;

	let canManage = $derived(auth.hasPermission('assets', 'manage'));

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
			total = result.total || 0;

			// Sort by updated_at for recent rules (most recent first)
			const sortedByRecent = [...allRules].sort(
				(a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
			);

			recentRules = sortedByRecent.slice(0, 4);
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

<div class="h-full overflow-y-auto">
	<div class="max-w-6xl mx-auto px-6 py-6">
		<!-- Header -->
		<div class="flex items-center justify-between mb-6">
			<div>
				<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
					{m.assetrules_heading()}
				</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
					{m.assetrules_subheading()}
				</p>
			</div>
			{#if canManage}
				<Button
					click={() => goto(resolve('/asset-rules/new'))}
					icon="material-symbols:add"
					text={m.assetrules_new_rule_button()}
					variant="filled"
				/>
			{/if}
		</div>

		{#if error}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-100 px-4 py-3 rounded-lg mb-6"
			>
				{error}
			</div>
		{:else if isLoading}
			<!-- Loading State -->
			<div class="space-y-6">
				<!-- Featured Cards Skeleton -->
				<div>
					<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-32 mb-4"></div>
					<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
						{#each Array(4) as _, i (i)}
							<div
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 animate-pulse"
							>
								<div class="h-10 w-10 bg-gray-200 dark:bg-gray-700 rounded-lg mb-3"></div>
								<div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-3/4 mb-2"></div>
								<div class="h-3 bg-gray-200 dark:bg-gray-700 rounded w-full mb-3"></div>
								<div class="flex gap-2">
									<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-16"></div>
									<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-12"></div>
								</div>
							</div>
						{/each}
					</div>
				</div>
				<!-- List Skeleton -->
				<div>
					<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-24 mb-4"></div>
					{#each Array(5) as _, i (i)}
						<div
							class="bg-white dark:bg-gray-800 border-b border-gray-100 dark:border-gray-700 p-3 animate-pulse"
						>
							<div class="flex items-center gap-3">
								<div class="h-6 w-6 bg-gray-200 dark:bg-gray-700 rounded"></div>
								<div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-48"></div>
								<div class="flex-1"></div>
								<div class="h-3 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else if rules.length === 0 && !searchQuery}
			<!-- Empty State -->
			<div class="flex flex-col items-center justify-center py-16">
				<div
					class="w-20 h-20 rounded-2xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center mb-4"
				>
					<IconifyIcon
						icon="material-symbols:rule-settings"
						class="text-4xl text-gray-400 dark:text-gray-500"
					/>
				</div>
				<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
					{m.assetrules_empty_heading()}
				</h2>
				<p class="text-sm text-gray-500 dark:text-gray-400 text-center max-w-md mb-6">
					{m.assetrules_empty_description()}
				</p>
				{#if canManage}
					<Button
						click={() => goto(resolve('/asset-rules/new'))}
						icon="material-symbols:add"
						text={m.assetrules_create_first_button()}
						variant="filled"
					/>
				{/if}
			</div>
		{:else}
			<!-- Recently Updated Section -->
			{#if !searchQuery && recentRules.length > 0}
				<section class="mb-8">
					<h2
						class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-4"
					>
						{m.assetrules_recently_updated_heading()}
					</h2>
					<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
						{#each recentRules as rule (rule.id)}
							<button
								onclick={() => goto(resolve(`/asset-rules/${rule.id}`))}
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 text-left hover:shadow-lg hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all group"
							>
								<div
									class="w-10 h-10 rounded-lg bg-gradient-to-br from-earthy-terracotta-100 to-earthy-terracotta-200 dark:from-earthy-terracotta-900/50 dark:to-earthy-terracotta-800/30 flex items-center justify-center mb-3"
								>
									<IconifyIcon
										icon="material-symbols:rule-settings"
										class="text-xl text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
									/>
								</div>
								<h3
									class="font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors mb-1"
								>
									{rule.name}
								</h3>
								{#if rule.description}
									<p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 mb-3">
										{rule.description}
									</p>
								{:else}
									<p class="text-xs text-gray-400 dark:text-gray-500 italic mb-3">
										{m.assetrules_no_description()}
									</p>
								{/if}
								<div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
									{#if rule.links?.length > 0}
										<span class="flex items-center gap-0.5">
											<IconifyIcon icon="material-symbols:link" class="w-3.5 h-3.5" />
											{rule.links.length}
										</span>
									{/if}
									{#if rule.term_ids?.length > 0}
										<span class="flex items-center gap-0.5">
											<IconifyIcon icon="material-symbols:book" class="w-3.5 h-3.5" />
											{rule.term_ids.length}
										</span>
									{/if}
									<span class="flex-1"></span>
									<span class="text-gray-400 dark:text-gray-500">
										{relativeTime(rule.updated_at)}
									</span>
								</div>
							</button>
						{/each}
					</div>
				</section>
			{/if}

			<!-- Search -->
			<div class="mb-6">
				<div class="relative">
					<IconifyIcon
						icon="material-symbols:search"
						class="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400"
					/>
					<input
						type="text"
						value={searchQuery}
						oninput={handleSearch}
						placeholder={m.assetrules_search_placeholder()}
						class="w-full pl-10 pr-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
					/>
				</div>
			</div>

			{#if rules.length === 0 && searchQuery}
				<div class="flex flex-col items-center justify-center py-16">
					<div
						class="w-20 h-20 rounded-2xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center mb-4"
					>
						<IconifyIcon
							icon="material-symbols:rule-settings"
							class="text-4xl text-gray-400 dark:text-gray-500"
						/>
					</div>
					<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
						{m.assetrules_no_matching_heading()}
					</h2>
					<p class="text-sm text-gray-500 dark:text-gray-400 text-center max-w-md mb-6">
						{m.assetrules_no_matching_hint()}
					</p>
				</div>
			{:else}
				<!-- All Rules List -->
				<section>
					<div class="flex items-center justify-between mb-4">
						<h2
							class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider"
						>
							{m.assetrules_all_rules_heading()}
						</h2>
						<span class="text-xs text-gray-500 dark:text-gray-400">
							{m.assetrules_rule_count({ count: total })}
						</span>
					</div>
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
					>
						{#each rules as rule, index (rule.id)}
							<button
								onclick={() => goto(resolve(`/asset-rules/${rule.id}`))}
								class="w-full flex items-center gap-4 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group {index !==
								rules.length - 1
									? 'border-b border-gray-100 dark:border-gray-700'
									: ''}"
							>
								<div
									class="w-8 h-8 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center flex-shrink-0"
								>
									<IconifyIcon
										icon="material-symbols:rule-settings"
										class="text-sm text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
									/>
								</div>
								<div class="flex-1 min-w-0">
									<h3
										class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
									>
										{rule.name}
									</h3>
									{#if rule.description}
										<p class="text-xs text-gray-500 dark:text-gray-400 truncate">
											{rule.description}
										</p>
									{/if}
								</div>
								<div class="flex items-center gap-4 flex-shrink-0">
									{#if rule.links?.length > 0}
										<span
											class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1"
											title={m.assetrules_links_title()}
										>
											<IconifyIcon icon="material-symbols:link" class="w-3.5 h-3.5" />
											{rule.links.length}
										</span>
									{/if}
									{#if rule.term_ids?.length > 0}
										<span
											class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1"
											title={m.assetrules_terms_title()}
										>
											<IconifyIcon icon="material-symbols:book" class="w-3.5 h-3.5" />
											{rule.term_ids.length}
										</span>
									{/if}
									<span
										class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1"
										title={m.assetrules_matched_assets_title()}
									>
										<IconifyIcon icon="material-symbols:database" class="w-3.5 h-3.5" />
										{rule.membership_count || 0}
									</span>
									<span
										class="text-xs px-2 py-0.5 rounded-full {rule.is_enabled
											? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
											: 'bg-gray-100 dark:bg-gray-700 text-gray-500'}"
									>
										{rule.is_enabled ? m.common_active() : m.common_disabled()}
									</span>
									<span
										class="text-xs text-gray-400 dark:text-gray-500 w-16 text-right"
										title={rule.updated_at ? formatDate(rule.updated_at) : ''}
									>
										{relativeTime(rule.updated_at)}
									</span>
									<IconifyIcon
										icon="mdi:chevron-right"
										class="w-4 h-4 text-gray-400 dark:text-gray-500 opacity-0 group-hover:opacity-100 transition-opacity"
									/>
								</div>
							</button>
						{/each}
					</div>
				</section>
			{/if}
		{/if}
	</div>
</div>
