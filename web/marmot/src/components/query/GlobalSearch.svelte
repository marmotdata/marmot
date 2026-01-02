<script lang="ts">
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import Icon from '@iconify/svelte';
	import AssetIcon from '$lib/components/AssetIcon.svelte';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';
	import { createKeyboardNavigationState } from '$lib/keyboard';

	export let initialQuery = '';
	export let autofocus = false;
	export let onNavigate: (() => void) | undefined = undefined;

	interface SearchResult {
		type: 'asset' | 'glossary' | 'team' | 'data_product';
		id: string;
		name: string;
		description?: string;
		metadata?: Record<string, any>;
		url: string;
	}

	interface SearchResponse {
		results: SearchResult[];
		total: number;
	}

	let searchQuery = initialQuery;
	let inputElement: HTMLInputElement;
	let searchResults: SearchResult[] = [];
	let isLoading = false;
	let showResults = false;
	let selectedIndex = -1;
	let debounceTimer: ReturnType<typeof setTimeout>;

	$: searchQuery = initialQuery;

	$: {
		clearTimeout(debounceTimer);
		if (searchQuery.trim()) {
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
			goto(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
			if (onNavigate) {
				onNavigate();
			}
		}
	}

	const { handleKeydown: navKeydown } = createKeyboardNavigationState(
		() => searchResults,
		() => selectedIndex,
		(i) => (selectedIndex = i),
		{
			onSelect: navigateToResult,
			onEscape: () => {
				if (showResults) {
					showResults = false;
					selectedIndex = -1;
				} else {
					searchQuery = '';
					if (onNavigate) {
						onNavigate();
					}
				}
			}
		}
	);

	function handleKeydown(event: KeyboardEvent) {
		// Only handle navigation keys when results are showing
		if (showResults && ['ArrowDown', 'ArrowUp', 'Enter'].includes(event.key)) {
			navKeydown(event);
			return;
		}
		// Always handle Escape
		if (event.key === 'Escape') {
			navKeydown(event);
		}
	}

	function navigateToResult(result: SearchResult) {
		goto(result.url);
		if (onNavigate) {
			onNavigate();
		}
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

	function getTypeColor(type: string): string {
		const colorMap: Record<string, string> = {
			asset: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
			glossary: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300',
			team: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
			data_product:
				'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-300'
		};
		return colorMap[type] || 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300';
	}

	function getTypeLabel(type: string): string {
		if (type === 'data_product') return 'product';
		return type;
	}

	function getResultSubtitle(result: SearchResult): string {
		if (result.type === 'asset' && result.metadata?.type) {
			return result.metadata.type;
		}
		if (result.description) {
			return result.description;
		}
		return '';
	}
</script>

<form on:submit={handleSubmit} class="w-full">
	<div class="relative">
		<div class="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
			<Icon icon="material-symbols:search" class="w-5 h-5 text-gray-400" aria-hidden="true" />
		</div>
		<input
			bind:this={inputElement}
			bind:value={searchQuery}
			on:keydown={handleKeydown}
			on:focus={() => searchQuery && (showResults = true)}
			type="text"
			placeholder="Search assets, glossary, teams..."
			aria-label="Search assets, glossary, and teams"
			aria-autocomplete="list"
			aria-expanded={showResults && searchResults.length > 0}
			{autofocus}
			class="block w-full pl-10 pr-3 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 focus:border-transparent text-base"
		/>

		{#if showResults && searchResults.length > 0}
			<div
				class="absolute z-50 w-full mt-2 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 max-h-96 overflow-y-auto"
			>
				{#each searchResults as result, index}
					<button
						type="button"
						tabindex="-1"
						on:click={() => navigateToResult(result)}
						on:mouseenter={() => (selectedIndex = index)}
						class="w-full text-left px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-700 border-b border-gray-100 dark:border-gray-700 last:border-b-0 transition-colors {selectedIndex ===
						index
							? 'bg-blue-50 dark:bg-blue-900/20'
							: ''}"
					>
						<div class="flex items-center gap-3">
							{#if result.type === 'asset'}
								<div class="flex-shrink-0">
									<AssetIcon
										assetType={result.metadata?.type}
										providers={result.metadata?.providers || []}
										size="md"
									/>
								</div>
							{:else if result.type === 'data_product'}
								<div
									class="flex-shrink-0 w-8 h-8 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center overflow-hidden"
								>
									{#if result.metadata?.icon_url}
										<AuthenticatedImage
											src={result.metadata.icon_url}
											alt="{result.name} icon"
											class="w-full h-full object-cover"
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
									class="text-xl text-gray-600 dark:text-gray-400 flex-shrink-0"
								/>
							{/if}
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									<p class="font-medium text-gray-900 dark:text-white truncate">
										{result.name}
									</p>
									<span
										class="flex-shrink-0 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getTypeColor(
											result.type
										)}"
									>
										{getTypeLabel(result.type)}
									</span>
								</div>
								{#if getResultSubtitle(result)}
									<p class="text-sm text-gray-600 dark:text-gray-400 truncate mt-0.5">
										{getResultSubtitle(result)}
									</p>
								{/if}
							</div>
						</div>
					</button>
				{/each}
				<div
					class="px-4 py-2 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700"
				>
					<p class="text-xs text-gray-500 dark:text-gray-400">
						Press <kbd
							class="px-1 py-0.5 text-xs font-semibold bg-gray-200 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded"
							>Enter</kbd
						> to see all results
					</p>
				</div>
			</div>
		{/if}

		{#if isLoading}
			<div class="absolute right-3 top-1/2 -translate-y-1/2">
				<Icon icon="mdi:loading" class="text-xl text-gray-400 animate-spin" />
			</div>
		{/if}
	</div>
</form>
