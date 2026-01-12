<script lang="ts">
	import { onMount } from 'svelte';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-json';
	import 'prismjs/components/prism-sql';
	import 'prismjs/components/prism-yaml';

	export let code: unknown;
	export let language: string = 'json';

	let formatted: string;
	let element: HTMLElement;
	let copySuccess = false;
	let copyTimeout: ReturnType<typeof setTimeout>;

	$: {
		formatted = code ? (typeof code === 'string' ? code : JSON.stringify(code, null, 2)) : '';
	}

	$: if (element && formatted) {
		setTimeout(() => {
			Prism.highlightElement(element);
		}, 0);
	}

	onMount(() => {
		if (element) {
			Prism.highlightElement(element);
		}
	});

	async function copyToClipboard() {
		try {
			await navigator.clipboard.writeText(formatted);
			copySuccess = true;

			if (copyTimeout) clearTimeout(copyTimeout);

			copyTimeout = setTimeout(() => {
				copySuccess = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy:', err);
		}
	}
</script>

<div class="code-block bg-gray-50 dark:bg-gray-800 rounded-lg">
	<div class="relative">
		<button
			on:click={copyToClipboard}
			class="absolute top-2 right-2 z-10 inline-flex items-center px-2 py-1 text-xs font-medium rounded
   		{copySuccess
				? 'bg-green-100 dark:bg-green-900/20 text-green-800 dark:text-green-100'
				: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}"
		>
			{#if copySuccess}
				<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M5 13l4 4L19 7"
					/>
				</svg>
				Copied!
			{:else}
				<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
					/>
				</svg>
				Copy
			{/if}
		</button>

		<pre class="p-6 overflow-x-auto"><code bind:this={element} class="language-{language}"
				>{formatted}</code
			></pre>
	</div>
</div>

<style>
	pre {
		margin: 0;
		background: transparent !important;
	}
	code {
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

	/* Light theme - Earthy colors matching Docusaurus */
	.code-block :global(code[class*='language-']),
	.code-block :global(pre[class*='language-']) {
		color: #1f2937;
	}

	.code-block :global(.token.comment),
	.code-block :global(.token.prolog),
	.code-block :global(.token.doctype),
	.code-block :global(.token.cdata) {
		color: #4a674a;
		font-style: italic;
	}

	.code-block :global(.token.namespace) {
		opacity: 0.7;
	}

	.code-block :global(.token.string),
	.code-block :global(.token.attr-value) {
		color: #35593b;
	}

	.code-block :global(.token.punctuation),
	.code-block :global(.token.operator) {
		color: #4a674a;
	}

	.code-block :global(.token.entity),
	.code-block :global(.token.url),
	.code-block :global(.token.symbol),
	.code-block :global(.token.number),
	.code-block :global(.token.boolean),
	.code-block :global(.token.variable),
	.code-block :global(.token.constant),
	.code-block :global(.token.property),
	.code-block :global(.token.regex),
	.code-block :global(.token.inserted) {
		color: #7b5935;
	}

	.code-block :global(.token.atrule),
	.code-block :global(.token.keyword),
	.code-block :global(.token.attr-name),
	.code-block :global(.token.selector) {
		color: #8d3718;
	}

	.code-block :global(.token.function),
	.code-block :global(.token.deleted),
	.code-block :global(.token.tag) {
		color: #b34822;
	}

	.code-block :global(.token.function-variable) {
		color: #b34822;
	}

	/* Dark theme - Brighter earthy tones matching Docusaurus */
	:global(.dark) .code-block :global(code[class*='language-']),
	:global(.dark) .code-block :global(pre[class*='language-']) {
		color: #f3f4f6;
	}

	:global(.dark) .code-block :global(.token.comment),
	:global(.dark) .code-block :global(.token.prolog),
	:global(.dark) .code-block :global(.token.doctype),
	:global(.dark) .code-block :global(.token.cdata) {
		color: #a8c5a8;
		font-style: italic;
	}

	:global(.dark) .code-block :global(.token.namespace) {
		opacity: 0.7;
	}

	:global(.dark) .code-block :global(.token.string),
	:global(.dark) .code-block :global(.token.attr-value) {
		color: #b9d9b9;
	}

	:global(.dark) .code-block :global(.token.punctuation),
	:global(.dark) .code-block :global(.token.operator) {
		color: #d1e5d1;
	}

	:global(.dark) .code-block :global(.token.entity),
	:global(.dark) .code-block :global(.token.url),
	:global(.dark) .code-block :global(.token.symbol),
	:global(.dark) .code-block :global(.token.number),
	:global(.dark) .code-block :global(.token.boolean),
	:global(.dark) .code-block :global(.token.variable),
	:global(.dark) .code-block :global(.token.constant),
	:global(.dark) .code-block :global(.token.property),
	:global(.dark) .code-block :global(.token.regex),
	:global(.dark) .code-block :global(.token.inserted) {
		color: #f0d97e;
	}

	:global(.dark) .code-block :global(.token.atrule),
	:global(.dark) .code-block :global(.token.keyword),
	:global(.dark) .code-block :global(.token.attr-name),
	:global(.dark) .code-block :global(.token.selector) {
		color: #ffa77d;
	}

	:global(.dark) .code-block :global(.token.function),
	:global(.dark) .code-block :global(.token.deleted),
	:global(.dark) .code-block :global(.token.tag) {
		color: #ffb899;
	}

	:global(.dark) .code-block :global(.token.function-variable) {
		color: #ffb899;
	}
</style>
