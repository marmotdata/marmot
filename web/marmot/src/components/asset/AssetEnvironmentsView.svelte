<script lang="ts">
	import type { Environment } from '$lib/assets/types';
	import MetadataView from '$components/shared/MetadataView.svelte';

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

<div class="space-y-4 p-4">
	{#if Object.keys(environments).length === 0}
		<div class="p-6 bg-earthy-brown-50 dark:bg-gray-800 rounded-lg">
			<p class="text-gray-500 dark:text-gray-400 italic">No environments available</p>
		</div>
	{:else}
		{#each Object.entries(environments) as [key, env]}
			<div
				class="bg-earthy-brown-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
			>
				<button
					class="w-full px-6 py-4 flex flex-col text-left hover:bg-earthy-brown-100 dark:hover:bg-gray-700 transition-colors"
					on:click={() => toggleEnvironment(key)}
					aria-expanded={expandedEnvironments[key]}
					aria-label="Toggle {key} environment details"
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
								aria-hidden="true"
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
							aria-hidden="true"
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
						<div class="mt-3 flex flex-wrap items-center gap-2">
							{#each getMetadataPreview(env.metadata) as [metaKey, metaValue]}
								<div class="inline-flex items-center gap-1.5 text-xs">
									<span class="font-medium text-gray-600 dark:text-gray-400" title={metaKey}>
										{metaKey}:
									</span>
									<span class="text-gray-900 dark:text-gray-100 font-mono">
										{formatValue(metaValue)}
									</span>
								</div>
								<span class="text-gray-300 dark:text-gray-600">•</span>
							{/each}
							<span class="text-xs text-gray-500 dark:text-gray-500 italic">
								{Object.keys(env.metadata).length}
								{Object.keys(env.metadata).length === 1 ? 'field' : 'fields'}
							</span>
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
