<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { marked } from 'marked';
	import { onMount } from 'svelte';

	marked.setOptions({
		gfm: true,
		breaks: true
	});

	export let mrn: string;
	let documentation = [];
	let loading = true;
	let error = null;

	async function fetchDocs() {
		try {
			loading = true;
			error = null;
			const encodedMrn = encodeURIComponent(mrn);
			const response = await fetchApi(`/assets/documentation/${encodedMrn}`);
			if (response.ok) {
				const data = await response.json();
				documentation = data || [];
			}
		} catch (e) {
			error = e;
		} finally {
			loading = false;
		}
	}

	$: if (mrn) {
		fetchDocs();
	}
</script>

<div
	class="mt-6 prose dark:prose-invert dark:prose-pre:bg-gray-800 prose-code:before:content-none prose-code:after:content-none max-w-none"
>
	{#if loading}
		<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
			<p class="text-gray-500 dark:text-gray-400">Loading documentation...</p>
		</div>
	{:else if error}
		<div class="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
			<p class="text-red-500 dark:text-red-400">Failed to load documentation</p>
		</div>
	{:else if documentation.length}
		{#each documentation as doc}
			<div class="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-6">
				<div class="mb-4 flex justify-between items-center">
					<span class="text-sm text-gray-500 dark:text-gray-400">Source: {doc.source}</span>
					<span class="text-sm text-gray-500 dark:text-gray-400">
						Updated: {new Date(doc.updated_at).toLocaleDateString()}
					</span>
				</div>
				<div>
					{@html marked(doc.content)}
				</div>
			</div>
		{/each}
	{:else}
		<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
			<p class="text-gray-500 dark:text-gray-400 italic">No documentation available</p>
		</div>
	{/if}
</div>

<style>
	:global(.dark .markdown-content) {
		color: #f3f4f6;
	}
	:global(.markdown-content h1) {
		@apply text-2xl font-bold mb-4 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content h2) {
		@apply text-xl font-bold mb-3 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content h3) {
		@apply text-lg font-bold mb-2 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content h4) {
		@apply text-base font-bold mb-2 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content p) {
		@apply mb-4 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content ul) {
		@apply list-disc pl-5 mb-4 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content ol) {
		@apply list-decimal pl-5 mb-4 text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content a) {
		@apply text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400 underline;
	}
	:global(.markdown-content code) {
		@apply bg-gray-100 dark:bg-gray-800 rounded px-1 py-0.5 font-mono text-sm text-gray-900 dark:text-gray-100;
	}
	:global(.markdown-content pre) {
		@apply bg-gray-100 dark:bg-gray-800 rounded p-4 overflow-x-auto mb-4;
	}
	:global(.markdown-content pre code) {
		@apply bg-transparent p-0;
	}
	:global(.markdown-content blockquote) {
		@apply pl-4 border-l-4 border-gray-200 dark:border-gray-600 my-4 text-gray-600 dark:text-gray-400 italic;
	}
	:global(.markdown-content blockquote p) {
		@apply mb-0;
	}
</style>
