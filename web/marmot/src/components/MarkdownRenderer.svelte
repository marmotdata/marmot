<script lang="ts">
	import { marked } from 'marked';
	import { onMount, afterUpdate } from 'svelte';
	import Prism from 'prismjs';
	import 'prism-themes/themes/prism-one-dark.css';
	import 'prismjs/components/prism-json';
	import 'prismjs/components/prism-sql';
	import 'prismjs/components/prism-python';
	import 'prismjs/components/prism-javascript';
	import 'prismjs/components/prism-typescript';
	import 'prismjs/components/prism-bash';
	import 'prismjs/components/prism-yaml';

	export let content: string;
	export let className: string = '';

	let renderedHtml = '';
	let containerElement: HTMLDivElement;

	$: {
		if (content) {
			renderedHtml = marked(content) as string;
		} else {
			renderedHtml = '';
		}
	}

	onMount(() => {
		marked.setOptions({
			breaks: true,
			gfm: true
		});
	});

	afterUpdate(() => {
		if (containerElement) {
			containerElement.querySelectorAll('pre code').forEach((block) => {
				Prism.highlightElement(block as HTMLElement);
			});
		}
	});
</script>

<div bind:this={containerElement} class="prose prose-sm dark:prose-invert max-w-none {className}">
	{@html renderedHtml}
</div>

<style>
	:global(.prose) {
		@apply text-gray-700 dark:text-gray-300;
	}
	:global(.prose h1) {
		@apply text-gray-900 dark:text-gray-100;
	}
	:global(.prose h2) {
		@apply text-gray-900 dark:text-gray-100;
	}
	:global(.prose h3) {
		@apply text-gray-900 dark:text-gray-100;
	}
	:global(.prose strong) {
		@apply text-gray-900 dark:text-gray-100 font-semibold;
	}
	:global(.prose code) {
		@apply bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-sm font-mono text-gray-900 dark:text-gray-100;
	}
	:global(.prose pre) {
		@apply bg-gray-50 dark:bg-gray-800 p-6 rounded-lg overflow-x-auto;
		margin: 1rem 0 !important;
	}
	:global(.prose pre code) {
		@apply bg-transparent px-0 py-0;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
		font-size: 0.875rem;
		background: transparent !important;
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}
	:global(.prose a) {
		@apply text-orange-600 dark:text-orange-400 no-underline hover:underline;
	}
</style>
