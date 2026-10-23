<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import Icon from '@iconify/svelte';
	import AssetIcon from '$lib/components/AssetIcon.svelte';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import { createKeyboardNavigationState } from '$lib/keyboard';
	import { m } from '$lib/paraglide/messages';
	import { auth } from '$lib/stores/auth';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { visibleItems } from '$lib/settings/sections';

	let {
		initialQuery = '',
		autofocus = false,
		onNavigate
	}: {
		initialQuery?: string;
		autofocus?: boolean;
		onNavigate?: () => void;
	} = $props();

	interface SearchResult {
		type: 'asset' | 'glossary' | 'team' | 'data_product';
		id: string;
		name: string;
		description?: string;
		metadata?: Record<string, unknown>;
		url: string;
	}

	interface SearchResponse {
		results: SearchResult[];
		total: number;
	}

	interface NavEntry {
		href: string;
		label: string;
		icon: string;
		/** Where the page lives, shown on the right; empty for top-level pages. */
		context: string;
	}

	type Item = { kind: 'nav'; entry: NavEntry } | { kind: 'result'; result: SearchResult };

	const MAX_NAV = 5;

	// svelte-ignore state_referenced_locally
	let searchQuery = $state(initialQuery);
	let inputElement: HTMLInputElement | undefined = $state();
	let searchResults = $state<SearchResult[]>([]);
	let isLoading = $state(false);
	let showResults = $state(false);
	let selectedIndex = $state(-1);
	let debounceTimer: ReturnType<typeof setTimeout>;

	// Every page the caller may open, from the same definitions the top bar and the Settings sub-navigation read, so the palette never offers a page they cannot see
	let navEntries = $derived.by((): NavEntry[] => {
		const entries: NavEntry[] = [
			{ href: '/discover', label: m.nav_discover(), icon: 'material-symbols:database', context: '' }
		];
		if ($auth && auth.hasPermission('assets', 'query')) {
			entries.push({
				href: '/query',
				label: m.nav_query(),
				icon: 'material-symbols:terminal',
				context: ''
			});
		}
		entries.push(
			{
				href: '/insights',
				label: m.nav_insights(),
				icon: 'material-symbols:insights',
				context: ''
			},
			{
				href: '/runs',
				label: m.nav_runs(),
				icon: 'material-symbols:play-circle-outline',
				context: ''
			},
			{ href: '/glossary', label: m.nav_glossary(), icon: 'material-symbols:book', context: '' },
			{
				href: '/products',
				label: m.nav_data_products(),
				icon: 'material-symbols:inventory-2',
				context: ''
			},
			{
				href: '/asset-rules',
				label: m.nav_asset_rules(),
				icon: 'material-symbols:rule-settings',
				context: ''
			}
		);
		if (holdsAtRoot($myAccess, 'access:view')) {
			entries.push({
				href: '/query-audit',
				label: m.nav_query_audit(),
				icon: 'material-symbols:history',
				context: m.nav_governance()
			});
		}
		const settings = visibleItems($myAccess);
		if (settings.length > 0) {
			entries.push({
				href: '/settings',
				label: m.nav_settings(),
				icon: 'material-symbols:settings-outline',
				context: ''
			});
			for (const item of settings) {
				entries.push({
					href: item.href,
					label: item.label(),
					icon: item.icon,
					context: `${m.nav_settings()} ${item.keywords()}`
				});
			}
		}
		if ($auth) {
			entries.push(
				{
					href: '/account',
					label: m.nav_account(),
					icon: 'material-symbols:account-circle-outline',
					context: m.account_profile_keywords()
				},
				{
					href: '/account/api-keys',
					label: m.account_api_keys(),
					icon: 'material-symbols:key-outline',
					context: `${m.nav_account()} ${m.account_api_keys_keywords()}`
				}
			);
		}
		return entries;
	});

	// A page matches when every word typed appears in its name or where it lives, so "settings roles" and "rol" both find Roles
	let navMatches = $derived.by(() => {
		const words = searchQuery.trim().toLowerCase().split(/\s+/).filter(Boolean);
		if (words.length === 0) return [];
		return navEntries
			.filter((entry) => {
				const haystack = `${entry.label} ${entry.context}`.toLowerCase();
				return words.every((w) => haystack.includes(w));
			})
			.slice(0, MAX_NAV);
	});

	let items = $derived<Item[]>([
		...navMatches.map((entry): Item => ({ kind: 'nav', entry })),
		...searchResults.map((result): Item => ({ kind: 'result', result }))
	]);

	let open = $derived(showResults && items.length > 0);

	function scheduleSearch() {
		clearTimeout(debounceTimer);
		selectedIndex = -1;
		if (searchQuery.trim()) {
			// Page matches are local, so they show at once while the catalog search waits for typing to settle
			showResults = true;
			debounceTimer = setTimeout(() => {
				fetchPreview();
			}, 300);
		} else {
			searchResults = [];
			showResults = false;
		}
	}

	export function focus() {
		inputElement?.focus();
	}

	async function fetchPreview() {
		if (!searchQuery.trim()) {
			searchResults = [];
			showResults = false;
			return;
		}

		try {
			isLoading = true;
			const response = await fetchApi(`/search?q=${encodeURIComponent(searchQuery)}&limit=5`);
			const data: SearchResponse = await response.json();
			searchResults = data.results || [];
			showResults = true;
			selectedIndex = -1;
		} catch (error) {
			console.error('Search preview error:', error);
			searchResults = [];
		} finally {
			isLoading = false;
		}
	}

	function handleSubmit(event: Event) {
		event.preventDefault();
		if (searchQuery.trim()) {
			goto(resolve(`/search?q=${encodeURIComponent(searchQuery.trim())}`));
			onNavigate?.();
		}
	}

	const { handleKeydown: navKeydown } = createKeyboardNavigationState(
		() => items,
		() => selectedIndex,
		(i) => (selectedIndex = i),
		{
			onSelect: select,
			onEscape: () => {
				if (showResults) {
					showResults = false;
					selectedIndex = -1;
				} else {
					searchQuery = '';
					onNavigate?.();
				}
			}
		}
	);

	function handleKeydown(event: KeyboardEvent) {
		// Only handle navigation keys when results are showing
		if (open && ['ArrowDown', 'ArrowUp', 'Enter'].includes(event.key)) {
			navKeydown(event);
			return;
		}
		// Always handle Escape
		if (event.key === 'Escape') {
			navKeydown(event);
		}
	}

	function select(item: Item) {
		goto(resolve((item.kind === 'nav' ? item.entry.href : item.result.url) as '/'));
		onNavigate?.();
	}

	function getTypeIcon(type: string): string {
		const iconMap: Record<string, string> = {
			asset: 'mdi:database',
			glossary: 'mdi:book-open-variant',
			team: 'mdi:account-group',
			data_product: 'mdi:package-variant-closed'
		};
		return iconMap[type] || 'mdi:file-document';
	}

	function getTypeTone(type: string): 'info' | 'neutral' | 'success' | 'brand' {
		const toneMap: Record<string, 'info' | 'neutral' | 'success' | 'brand'> = {
			asset: 'info',
			glossary: 'neutral',
			team: 'success',
			data_product: 'brand'
		};
		return toneMap[type] ?? 'neutral';
	}

	// Known result types map to translated badge labels, unknown types fall back to the raw string
	function getTypeLabel(type: string): string {
		const labelMap: Record<string, () => string> = {
			asset: m.search_type_asset,
			glossary: m.search_type_glossary,
			team: m.search_type_team,
			data_product: m.search_type_product
		};
		return labelMap[type] ? labelMap[type]() : type;
	}

	function getResultSubtitle(result: SearchResult): string {
		if (result.type === 'asset' && typeof result.metadata?.type === 'string') {
			return result.metadata.type;
		}
		if (result.description) {
			return result.description;
		}
		return '';
	}

	function rowClass(index: number): string {
		return selectedIndex === index ? 'bg-gray-100 dark:bg-gray-700/60' : '';
	}
</script>

<form onsubmit={handleSubmit} class="w-full">
	<div class="relative">
		<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
			<Icon
				icon="material-symbols:search"
				class="h-5 w-5 text-gray-600 dark:text-gray-400"
				aria-hidden="true"
			/>
		</div>
		<!-- svelte-ignore a11y_autofocus -->
		<input
			bind:this={inputElement}
			bind:value={searchQuery}
			oninput={scheduleSearch}
			onkeydown={handleKeydown}
			onfocus={() => searchQuery && (showResults = true)}
			type="text"
			placeholder={m.search_input_placeholder()}
			aria-label={m.search_input_aria_label()}
			role="combobox"
			aria-autocomplete="list"
			aria-expanded={open}
			aria-controls="global-search-results"
			{autofocus}
			class="block w-full rounded-lg border border-gray-300 bg-white py-3 pl-10 pr-3 text-base text-gray-900 placeholder-gray-500 focus:border-transparent focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:border-gray-600 dark:bg-gray-900 dark:text-white dark:placeholder-gray-400"
		/>

		{#if open}
			<div
				id="global-search-results"
				class="absolute z-50 mt-2 max-h-96 w-full overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
			>
				{#if navMatches.length > 0}
					<div
						class="border-b border-gray-100 py-1 dark:border-gray-700"
						role="group"
						aria-label={m.search_goto_heading()}
					>
						<p
							class="px-4 pb-1 pt-1.5 text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500"
						>
							{m.search_goto_heading()}
						</p>
						{#each navMatches as entry, index (entry.href)}
							<button
								type="button"
								tabindex="-1"
								onclick={() => select({ kind: 'nav', entry })}
								onmouseenter={() => (selectedIndex = index)}
								class="flex w-full items-center gap-3 px-4 py-1.5 text-left text-sm transition-colors hover:bg-gray-50 dark:hover:bg-gray-700 {rowClass(
									index
								)}"
							>
								<Icon icon={entry.icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
								<span class="min-w-0 flex-1 truncate text-gray-900 dark:text-gray-100">
									{entry.label}
								</span>
								{#if entry.context}
									<span class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
										{entry.context}
									</span>
								{/if}
								<Icon
									icon="material-symbols:arrow-forward"
									class="h-4 w-4 flex-shrink-0 text-gray-300 dark:text-gray-600"
								/>
							</button>
						{/each}
					</div>
				{/if}

				{#each searchResults as result, i (result.id)}
					{@const index = navMatches.length + i}
					<button
						type="button"
						tabindex="-1"
						onclick={() => select({ kind: 'result', result })}
						onmouseenter={() => (selectedIndex = index)}
						class="w-full border-b border-gray-100 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-700 {rowClass(
							index
						)}"
					>
						<div class="flex items-center gap-3">
							{#if result.type === 'asset'}
								<div class="flex-shrink-0">
									<AssetIcon
										assetType={result.metadata?.type as string | undefined}
										providers={(result.metadata?.providers as string[] | undefined) || []}
										size="md"
									/>
								</div>
							{:else if result.type === 'data_product'}
								<div
									class="flex h-8 w-8 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30"
								>
									{#if result.metadata?.icon_url}
										<AuthenticatedImage
											src={result.metadata.icon_url as string}
											alt={m.search_product_icon_alt({ name: result.name })}
											class="h-full w-full object-cover"
										/>
									{:else}
										<Icon
											icon="mdi:package-variant-closed"
											class="text-sm text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
										/>
									{/if}
								</div>
							{:else}
								<Icon
									icon={getTypeIcon(result.type)}
									class="flex-shrink-0 text-xl text-gray-600 dark:text-gray-400"
								/>
							{/if}
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2">
									<p class="truncate font-medium text-gray-900 dark:text-white">
										{result.name}
									</p>
									<Pill tone={getTypeTone(result.type)}>{getTypeLabel(result.type)}</Pill>
								</div>
								{#if getResultSubtitle(result)}
									<p class="mt-0.5 truncate text-sm text-gray-600 dark:text-gray-400">
										{getResultSubtitle(result)}
									</p>
								{/if}
							</div>
						</div>
					</button>
				{/each}
				{#if searchResults.length > 0}
					<div
						class="border-t border-gray-200 bg-gray-50 px-4 py-2 dark:border-gray-700 dark:bg-gray-900/50"
					>
						<p class="text-xs text-gray-600 dark:text-gray-400">
							{m.search_press_enter_hint()}
						</p>
					</div>
				{/if}
			</div>
		{/if}

		{#if isLoading}
			<div class="absolute right-3 top-1/2 -translate-y-1/2">
				<Icon icon="mdi:loading" class="animate-spin text-xl text-gray-600 dark:text-gray-400" />
			</div>
		{/if}
	</div>
</form>
