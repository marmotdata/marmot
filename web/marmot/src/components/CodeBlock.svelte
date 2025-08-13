<script lang="ts">
	import { onMount } from 'svelte';
	import Prism from 'prismjs';
	import 'prism-themes/themes/prism-one-dark.css';
	import 'prismjs/components/prism-json';
	import 'prismjs/components/prism-sql';

	export let code: any;
	export let language: string = 'json';

	let formatted: string;
	let element: HTMLElement;
	let copySuccess = false;
	let copyTimeout: NodeJS.Timeout;

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

<div class="bg-gray-50 dark:bg-gray-800 rounded-lg">
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

		<pre class="p-6 text-gray-900 dark:text-gray-100 overflow-x-auto"><code
				bind:this={element}
				class="language-{language}">{formatted}</code
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
</style>
