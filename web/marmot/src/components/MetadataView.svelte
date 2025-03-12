<script lang="ts">
	import Arrow from './Arrow.svelte';

	export let metadata: any;
	export let depth = 0;
	export let maxDepth = Infinity;
	export let maxCharLength = Infinity;
	export let showDetailsLink: string | undefined = undefined;
	export let standalone = true;

	function isObject(value: any): boolean {
		return typeof value === 'object' && value !== null && !Array.isArray(value);
	}

	function isArray(value: any): boolean {
		return Array.isArray(value);
	}

	function truncateText(text: string, maxLength: number): string {
		if (maxLength === Infinity) return text;
		return text.length > maxLength ? text.substring(0, maxLength - 3) + '...' : text;
	}

	function getValueClass(value: any): string {
		if (typeof value === 'boolean') {
			return value
				? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-100'
				: 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-100';
		}
		if (typeof value === 'number')
			return 'bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-100';
		if (typeof value === 'string')
			return 'bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-100';
		return '';
	}

	let expandedDetails: { [key: number]: boolean } = {};

	function toggleDetails(index: number) {
		expandedDetails[index] = !expandedDetails[index];
		expandedDetails = expandedDetails;
	}
</script>

{#if standalone}
	<div
		class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm dark:shadow-md"
	>
		<div class="overflow-x-auto">
			<svelte:self
				{metadata}
				{depth}
				{maxDepth}
				{maxCharLength}
				{showDetailsLink}
				standalone={false}
			/>
		</div>
	</div>
{:else}
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-white dark:bg-gray-800"
				>
					Key
				</th>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider bg-white dark:bg-gray-800"
				>
					Value
				</th>
			</tr>
		</thead>
		<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
			{#each Object.entries(metadata) as [key, value], i}
				<tr class="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
					<td
						class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100"
						>{key}</td
					>
					<td class="px-6 py-4 text-sm">
						{#if isObject(value)}
							{#if showDetailsLink}
								<a
									href={showDetailsLink}
									class="inline-flex items-center text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300"
								>
									<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
										/>
									</svg>
									View in Full Page
								</a>
							{:else if depth < maxDepth}
								<details class="group" open={expandedDetails[i]}>
									<summary
										class="cursor-pointer text-gray-700 hover:text-blue-600 dark:text-gray-300 dark:hover:text-blue-400 flex items-center"
										on:click|preventDefault={() => toggleDetails(i)}
									>
										<Arrow expanded={expandedDetails[i]} />
										<span class="ml-1">View Details</span>
									</summary>
									<div class="mt-2 pl-6 border-l-2 border-gray-200 dark:border-gray-600">
										<svelte:self
											metadata={value}
											depth={depth + 1}
											{maxDepth}
											{maxCharLength}
											{showDetailsLink}
											standalone={false}
										/>
									</div>
								</details>
							{/if}
						{:else if isArray(value)}
							<div class="flex flex-wrap gap-2">
								{#each value as item}
									<span
										class="px-2 py-1 text-sm bg-amber-100 dark:bg-amber-900 text-amber-800 dark:text-amber-100 rounded-full"
									>
										{truncateText(item?.toString() || '', maxCharLength)}
									</span>
								{/each}
							</div>
						{:else}
							<span class="px-2 py-1 text-sm rounded-full {getValueClass(value)}">
								{truncateText(value?.toString() || '', maxCharLength)}
							</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}
