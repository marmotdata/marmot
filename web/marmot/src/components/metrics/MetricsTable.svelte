<script lang="ts">
	import { fetchApi } from '$lib/api';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';

	interface MetricItem {
		id: string;
		name: string;
		subtitle?: string;
		count: number;
		icon?: string;
		clickable?: boolean;
		[key: string]: any;
	}

	export let startDate: string;
	export let endDate: string;
	export let timeRangeLabel: string;
	export let endpoint: string;
	export let title: string;
	export let icon: string;
	export let emptyIcon: string;
	export let emptyMessage: string;
	export let emptyDescription: string;
	export let countLabel: string;
	export let limit: number = 10;
	export let onItemClick: ((item: MetricItem) => void) | null = null;
	export let transformData: ((rawData: any[]) => MetricItem[]) | null = null;

	let items: MetricItem[] = [];
	let loading = true;
	let error: string | null = null;

	function handleItemClick(item: MetricItem) {
		if (onItemClick && item.clickable !== false) {
			onItemClick(item);
		}
	}

	async function fetchData() {
		try {
			loading = true;
			error = null;

			const start = encodeURIComponent(startDate);
			const end = encodeURIComponent(endDate);

			const response = await fetchApi(`${endpoint}?start=${start}&end=${end}&limit=${limit}`);

			if (!response.ok) {
				throw new Error(`Failed to fetch data`);
			}

			const rawData = await response.json();

			// Handle null or undefined response
			if (!rawData) {
				items = [];
				return;
			}

			// Transform data if transformer is provided, otherwise use raw data
			if (transformData) {
				items = transformData(rawData);
			} else {
				// Ensure rawData is an array
				items = Array.isArray(rawData) ? rawData : [];
			}
		} catch (err) {
			console.error(`Error fetching data:`, err);
			error = err instanceof Error ? err.message : `Failed to load data`;
			items = [];
		} finally {
			loading = false;
		}
	}

	// Reactive statement to fetch data when dates change
	$: if (startDate && endDate && endpoint) {
		fetchData();
	}
</script>

<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
	<div class="px-6 py-3 border-b border-gray-200 dark:border-gray-700">
		<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
			<IconifyIcon
				{icon}
				class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
			/>
			{title}
		</h2>
		<p class="text-sm text-gray-600 dark:text-gray-400 mt-0.5">
			{title} in {timeRangeLabel.toLowerCase()}
		</p>
	</div>

	<div class="p-2.5">
		{#if loading}
			<div class="flex items-center justify-center h-30">
				<div
					class="animate-spin rounded-full h-7 w-7 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if error}
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-2.5"
			>
				<div class="flex items-center gap-2">
					<IconifyIcon icon="mdi:alert-circle" class="w-4 h-4 text-red-600 dark:text-red-400" />
					<p class="text-red-600 dark:text-red-400 text-sm">{error}</p>
				</div>
			</div>
		{:else if items.length === 0}
			<div class="flex items-center justify-center py-8">
				<div class="text-center">
					<IconifyIcon icon={emptyIcon} class="w-8 h-8 text-gray-400 mx-auto mb-2" />
					<p class="text-gray-500 dark:text-gray-400 text-sm mb-1">{emptyMessage}</p>
					<p class="text-gray-400 dark:text-gray-500 text-xs">
						{emptyDescription}
					</p>
				</div>
			</div>
		{:else}
			<div class="space-y-1">
				{#each items as item, index}
					<div
						class="flex items-center justify-between p-2 bg-earthy-brown-50 dark:bg-gray-700/50 rounded-md {onItemClick &&
						item.clickable !== false
							? 'hover:bg-earthy-brown-100 dark:hover:bg-gray-600/50 cursor-pointer transition-colors'
							: ''}"
						title={item.name}
						on:click={() => handleItemClick(item)}
						role={onItemClick && item.clickable !== false ? 'button' : undefined}
						tabindex={onItemClick && item.clickable !== false ? 0 : undefined}
						on:keydown={(e) => {
							if (onItemClick && item.clickable !== false && (e.key === 'Enter' || e.key === ' ')) {
								e.preventDefault();
								handleItemClick(item);
							}
						}}
					>
						<div class="flex items-center gap-2 flex-1 min-w-0">
							<div
								class="flex items-center justify-center w-5 h-5 bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700 rounded-full text-xs font-medium flex-shrink-0"
							>
								{index + 1}
							</div>
							{#if item.icon}
								<Icon name={item.icon} showLabel={false} size="sm" class="flex-shrink-0" />
							{/if}
							<div class="flex-1 min-w-0">
								<p
									class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
									title={item.name}
								>
									{item.name}
								</p>
								{#if item.subtitle}
									<p
										class="text-xs text-gray-500 dark:text-gray-400 truncate"
										title={item.subtitle}
									>
										{item.subtitle}
									</p>
								{/if}
							</div>
						</div>
						<div class="text-right flex-shrink-0 ml-1.5">
							<p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{item.count.toLocaleString()}
							</p>
							<p class="text-xs text-gray-500 dark:text-gray-400">{countLabel}</p>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
