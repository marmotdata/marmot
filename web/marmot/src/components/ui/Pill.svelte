<script lang="ts">
	import type { Snippet } from 'svelte';

	// A status or category label. Colour only says something when the state is worth noticing.
	let {
		tone = 'neutral',
		children
	}: {
		tone?: 'neutral' | 'brand' | 'info' | 'success' | 'warning' | 'danger';
		children: Snippet;
	} = $props();

	const tones = {
		neutral: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300',
		brand:
			'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/30 dark:text-earthy-terracotta-300',
		info: 'bg-earthy-blue-100 text-earthy-blue-800 dark:bg-earthy-blue-900/30 dark:text-earthy-blue-300',
		success:
			'bg-earthy-green-100 text-earthy-green-800 dark:bg-earthy-green-900/30 dark:text-earthy-green-300',
		warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
		danger: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
	};
</script>

<span
	class="inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium {tones[
		tone
	]}"
>
	{@render children()}
</span>
