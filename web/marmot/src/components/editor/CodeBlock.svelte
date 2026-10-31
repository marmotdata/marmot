<script lang="ts">
	import { onMount } from 'svelte';
	import Prism from 'prismjs';
	import { m } from '$lib/paraglide/messages';
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

<div class="code-block relative rounded-md">
	<div class="absolute top-space-2 right-space-2 z-10">
		<mm-button
			variant="ghost"
			size="sm"
			icon={copySuccess ? 'check' : 'content-copy'}
			onclick={copyToClipboard}>{copySuccess ? m.common_copied() : m.common_copy()}</mm-button
		>
	</div>
	<pre class="overflow-x-auto"><code bind:this={element} class="language-{language}"
			>{formatted}</code
		></pre>
</div>

<style>
	.code-block {
		background: var(--mm-code-bg);
		color: var(--mm-code-text);
		/* The code well is dark in both themes, so the copy button reads its ink from the code text colour. */
		--mm-text: var(--mm-code-text);
		--mm-text-2: var(--mm-code-text);
	}
	pre {
		margin: 0;
		padding: var(--space-4) var(--space-5);
		background: transparent !important;
	}
	code {
		font: var(--text-mono-editor);
		background: transparent !important;
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}

	.code-block :global(code[class*='language-']),
	.code-block :global(pre[class*='language-']) {
		color: var(--mm-code-text);
		text-shadow: none;
	}

	.code-block :global(.token.comment),
	.code-block :global(.token.prolog),
	.code-block :global(.token.doctype),
	.code-block :global(.token.cdata) {
		color: var(--mm-syn-comment);
		font-style: italic;
	}

	.code-block :global(.token.namespace) {
		opacity: 0.7;
	}

	.code-block :global(.token.string),
	.code-block :global(.token.attr-value) {
		color: var(--mm-syn-string);
	}

	.code-block :global(.token.punctuation),
	.code-block :global(.token.operator) {
		color: var(--mm-code-text);
		background: none;
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
		color: var(--mm-syn-number);
	}

	.code-block :global(.token.atrule),
	.code-block :global(.token.keyword),
	.code-block :global(.token.attr-name),
	.code-block :global(.token.selector) {
		color: var(--mm-syn-keyword);
	}

	.code-block :global(.token.function),
	.code-block :global(.token.function-variable),
	.code-block :global(.token.deleted),
	.code-block :global(.token.tag) {
		color: var(--mm-syn-function);
	}
</style>
