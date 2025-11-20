<script lang="ts">
	import MarkdownRenderer from './MarkdownRenderer.svelte';

	export let value: string;
	export let placeholder: string = '';
	export let rows: number = 4;
	export let disabled: boolean = false;
	export let showPreview: boolean = true;

	let previewMode = false;
</script>

<div class="space-y-2">
	{#if showPreview}
		<div class="flex gap-2 border-b border-gray-200 dark:border-gray-700">
			<button
				type="button"
				on:click={() => (previewMode = false)}
				class="px-3 py-1.5 text-sm font-medium {previewMode
					? 'text-gray-500 dark:text-gray-400'
					: 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700 border-b-2 border-earthy-terracotta-700 dark:border-earthy-terracotta-500'}"
			>
				Write
			</button>
			<button
				type="button"
				on:click={() => (previewMode = true)}
				class="px-3 py-1.5 text-sm font-medium {previewMode
					? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700 border-b-2 border-earthy-terracotta-700 dark:border-earthy-terracotta-500'
					: 'text-gray-500 dark:text-gray-400'}"
			>
				Preview
			</button>
		</div>
	{/if}

	{#if previewMode && showPreview}
		<div
			class="min-h-[100px] p-3 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-900"
		>
			{#if value}
				<MarkdownRenderer content={value} />
			{:else}
				<p class="text-sm text-gray-400 dark:text-gray-500 italic">Nothing to preview</p>
			{/if}
		</div>
	{:else}
		<textarea
			bind:value
			{placeholder}
			{rows}
			{disabled}
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 font-mono text-sm"
		></textarea>
		{#if showPreview}
			<p class="text-xs text-gray-500 dark:text-gray-400">
				Markdown is supported. Use **bold**, *italic*, `code`, and more.
			</p>
		{/if}
	{/if}
</div>
