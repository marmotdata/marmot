<script lang="ts">
	import type { Environment } from '$lib/assets/types';
	import MetadataView from './MetadataView.svelte';

	export let environments: Record<string, Environment>;

	let expandedEnvironments: Record<string, boolean> = {};
	let metadataUniquenessScores: Record<string, number> = {};

	// Calculate uniqueness scores when environments change
	$: {
		const allValues: Record<string, Set<string>> = {};
		const keyCount: Record<string, number> = {};

		// First pass: collect all unique values for each key
		Object.values(environments).forEach((env) => {
			Object.entries(env.metadata).forEach(([key, value]) => {
				if (!allValues[key]) {
					allValues[key] = new Set();
					keyCount[key] = 0;
				}
				allValues[key].add(JSON.stringify(value));
				keyCount[key]++;
			});
		});

		// Calculate uniqueness score (unique values / total occurrences)
		Object.keys(allValues).forEach((key) => {
			metadataUniquenessScores[key] = allValues[key].size / keyCount[key];
		});
	}

	function toggleEnvironment(envName: string) {
		expandedEnvironments[envName] = !expandedEnvironments[envName];
	}

	function getMetadataPreview(
		metadata: Record<string, any>,
		limit: number = 4
	): Array<[string, string | number | boolean | any[]]> {
		return Object.entries(metadata)
			.sort((a, b) => {
				// Sort by uniqueness score (higher score = more unique = comes first)
				return (metadataUniquenessScores[b[0]] || 0) - (metadataUniquenessScores[a[0]] || 0);
			})
			.slice(0, limit)
			.map(([key, value]) => [key, value]);
	}

	function getValueClass(value: any): string {
		if (typeof value === 'boolean') {
			return value ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800';
		}
		if (typeof value === 'number') return 'bg-blue-100 text-blue-800';
		if (Array.isArray(value)) return 'bg-earthy-terracotta-100 text-earthy-terracotta-700';
		return 'bg-gray-100 text-gray-800 dark:text-gray-200';
	}

	function formatValue(value: any): string {
		if (typeof value === 'object' && value !== null) {
			return Array.isArray(value) ? value.join(', ') : JSON.stringify(value);
		}
		return String(value);
	}
</script>

<div class="space-y-4">
	{#if Object.keys(environments).length === 0}
		<div class="p-4 bg-earthy-brown-50 dark:bg-gray-800 rounded-lg">
			<p class="text-gray-500 dark:text-gray-400 italic">No environments available</p>
		</div>
	{:else}
		{#each Object.entries(environments) as [key, env]}
			<div
				class="bg-earthy-brown-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
			>
				<button
					class="w-full px-4 py-3 flex flex-col text-left hover:bg-earthy-brown-100 dark:hover:bg-gray-700 transition-colors"
					on:click={() => toggleEnvironment(key)}
				>
					<div class="flex items-center justify-between">
						<div class="flex items-center space-x-3">
							<svg
								class="w-5 h-5 text-gray-400"
								xmlns="http://www.w3.org/2000/svg"
								fill="none"
								viewBox="0 0 24 24"
								stroke-width="1.5"
								stroke="currentColor"
							>
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z"
								/>
							</svg>
							<span class="font-bold text-gray-900 dark:text-gray-100 truncate max-w-[600px]"
								>{key}</span
							>
						</div>
						<svg
							class="flex-shrink-0 w-5 h-5 text-gray-400 transform transition-transform duration-200 {expandedEnvironments[
								key
							]
								? 'rotate-180'
								: ''}"
							fill="none"
							stroke="currentColor"
							viewBox="0 0 24 24"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M19 9l-7 7-7-7"
							/>
						</svg>
					</div>
					{#if !expandedEnvironments[key]}
						<div class="mt-3 grid grid-cols-2 gap-3">
							{#each getMetadataPreview(env.metadata) as [metaKey, metaValue]}
								<div class="flex items-baseline gap-2 text-sm">
									<span
										class="font-medium text-gray-900 dark:text-gray-100 truncate min-w-[80px] max-w-[150px]"
									>
										{metaKey}
									</span>
									{#if typeof metaValue === 'boolean'}
										<span
											class="px-2 py-1 text-sm rounded-full {metaValue
												? 'bg-green-100 dark:bg-green-900/20 text-green-800 dark:text-green-100'
												: 'bg-red-100 dark:bg-red-900/20 text-red-800 dark:text-red-100'}"
										>
											{formatValue(metaValue)}
										</span>
									{:else if Array.isArray(metaValue)}
										<span
											class="px-2 py-1 text-sm rounded-full bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
										>
											{formatValue(metaValue)}
										</span>
									{:else if typeof metaValue === 'number'}
										<span
											class="px-2 py-1 text-sm rounded-full bg-blue-100 dark:bg-blue-900/20 text-blue-800 dark:text-blue-100"
										>
											{formatValue(metaValue)}
										</span>
									{:else}
										<span
											class="px-2 py-1 text-sm rounded-full bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-100"
										>
											{formatValue(metaValue)}
										</span>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</button>
				{#if expandedEnvironments[key]}
					<div
						class="border-t border-gray-200 dark:border-gray-700 transform origin-top transition-all duration-200 ease-in-out"
					>
						<MetadataView metadata={env.metadata} standalone={false} />
					</div>
				{/if}
			</div>
		{/each}
	{/if}
</div>
