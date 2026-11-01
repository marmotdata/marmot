<script lang="ts">
	import type { Snippet } from 'svelte';

	// Kept for pages not yet moved to mm-callout directly: the same tones, with "error" as the callout's danger tone.
	let {
		tone = 'error',
		title,
		children
	}: {
		tone?: 'error' | 'warning' | 'info' | 'success';
		title?: string;
		children?: Snippet;
	} = $props();
</script>

<mm-callout class="block" tone={tone === 'error' ? 'danger' : tone} heading={title ?? ''}>
	{@render children?.()}
</mm-callout>
