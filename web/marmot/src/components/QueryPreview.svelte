<script lang="ts">
	import CodeBlock from './CodeBlock.svelte';

	export let query: string;
	export let queryLanguage: string = 'sql';
</script>

<div class="space-y-4">
	{#if query}
		<CodeBlock code={query} language={queryLanguage.toLowerCase()} />
	{:else}
		<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
			<p class="text-gray-500 dark:text-gray-400 italic">No query available</p>
		</div>
	{/if}
</div>
