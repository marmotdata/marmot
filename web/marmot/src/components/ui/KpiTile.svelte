<script lang="ts">
	import type { Snippet } from 'svelte';
	import { PANEL, LABEL } from '$lib/ui/classes';

	// One headline number. Tiles belong on overview pages only; lists of things are tables.
	let {
		label,
		value,
		sub,
		href,
		tone = 'default',
		children
	}: {
		label: string;
		value: string | number;
		sub?: string;
		href?: string;
		tone?: 'default' | 'warning' | 'danger' | 'success';
		children?: Snippet;
	} = $props();

	const toneClass = {
		default: 'text-gray-900 dark:text-gray-100',
		warning: 'text-amber-600 dark:text-amber-400',
		danger: 'text-red-600 dark:text-red-400',
		success: 'text-earthy-green-700 dark:text-earthy-green-500'
	};
</script>

<svelte:element
	this={href ? 'a' : 'div'}
	{href}
	class="{PANEL} block p-4 {href
		? 'transition hover:border-gray-300 dark:hover:border-gray-600'
		: ''}"
>
	<p class={LABEL}>{label}</p>
	<p class="mt-1 text-2xl font-semibold tabular-nums {toneClass[tone]}">{value}</p>
	{#if sub}<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{sub}</p>{/if}
	{#if children}<div class="mt-3">{@render children()}</div>{/if}
</svelte:element>
