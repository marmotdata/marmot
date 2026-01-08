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
		color: #4D4D4D;
	}
	:global(.dark .prose),
	:global(.dark .prose p),
	:global(.dark .prose li),
	:global(.dark .prose span) {
		color: #DFDFDF !important;
	}
	:global(.prose h1),
	:global(.prose h2),
	:global(.prose h3) {
		@apply text-gray-900;
	}
	:global(.dark .prose h1),
	:global(.dark .prose h2),
	:global(.dark .prose h3) {
		color: #F9F9F9 !important;
	}
	:global(.prose strong) {
		@apply text-gray-900 font-semibold;
	}
	:global(.dark .prose strong) {
		color: #F9F9F9 !important;
	}
	:global(.prose code) {
		@apply bg-gray-100 px-1 py-0.5 rounded text-sm font-mono text-gray-900;
	}
	:global(.dark .prose code) {
		@apply bg-gray-800;
		color: #F9F9F9 !important;
	}
	:global(.prose pre) {
		@apply bg-gray-50 p-6 rounded-lg overflow-x-auto;
		margin: 1rem 0 !important;
	}
	:global(.dark .prose pre) {
		@apply bg-gray-800;
	}
	:global(.prose pre code) {
		@apply bg-transparent px-0 py-0;
		font-family:
			ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
			monospace;
		font-size: 0.875rem;
		background: transparent !important;
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}
	:global(.prose a) {
		@apply text-earthy-terracotta-700 no-underline hover:underline;
	}
	:global(.dark .prose a) {
		@apply text-earthy-terracotta-400;
	}
</style>
