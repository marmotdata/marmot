<script lang="ts">
	import { marked } from 'marked';
	import { onMount, afterUpdate } from 'svelte';
	import Prism from 'prismjs';
	import { sanitizeHtml } from '$lib/sanitize';
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
			renderedHtml = sanitizeHtml(marked(content) as string);
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

<div bind:this={containerElement} class="prose max-w-none {className}">
	<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitized via DOMPurify in the reactive block above -->
	{@html renderedHtml}
</div>

<style>
	/* Prose follows the tokens so it switches with the theme on its own; links in running text are the one place burnt sienna is underlined. */
	:global(.prose) {
		--tw-prose-body: var(--mm-text);
		--tw-prose-headings: var(--mm-text);
		--tw-prose-bold: var(--mm-text);
		--tw-prose-links: var(--mm-primary);
		--tw-prose-bullets: var(--mm-text-3);
		--tw-prose-counters: var(--mm-text-2);
		--tw-prose-quotes: var(--mm-text-2);
		--tw-prose-quote-borders: var(--mm-rule);
		--tw-prose-hr: var(--mm-rule);
		--tw-prose-th-borders: var(--mm-rule);
		--tw-prose-td-borders: var(--mm-rule);
		--tw-prose-code: var(--mm-text);
		--tw-prose-captions: var(--mm-text-2);
		color: var(--mm-text);
		font: var(--text-body);
	}
	:global(.prose :where(h1, h2, h3, h4)) {
		color: var(--mm-text);
		letter-spacing: 0;
	}
	:global(.prose code) {
		background: var(--mm-sunken);
		color: var(--mm-text);
		padding: 0.1em 0.3em;
		border-radius: var(--radius-sm);
		font: var(--text-mono-dense);
	}
	:global(.prose code::before),
	:global(.prose code::after) {
		content: none;
	}
	:global(.prose pre) {
		background: var(--mm-code-bg);
		color: var(--mm-code-text);
		padding: var(--space-4);
		border-radius: var(--radius-md);
		overflow-x: auto;
		margin: 1rem 0 !important;
	}
	:global(.prose pre code) {
		background: transparent !important;
		color: inherit;
		padding: 0;
		font: var(--text-mono-dense);
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}
	:global(.prose a) {
		color: var(--mm-primary);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	:global(.prose a:hover) {
		color: var(--mm-primary-hover);
	}
</style>
