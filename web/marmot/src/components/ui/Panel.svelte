<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { PANEL } from '$lib/ui/classes';

	// A module on a page: an optional small header with an icon and a right-hand action, then the content. Flush panels hold tables and lists edge to edge.
	let {
		title,
		description,
		icon,
		flush = false,
		class: extra = '',
		action,
		children
	}: {
		title?: string;
		description?: string;
		icon?: string;
		flush?: boolean;
		class?: string;
		action?: Snippet;
		children: Snippet;
	} = $props();
</script>

<section class="{PANEL} {flush ? 'overflow-hidden' : 'p-5'} {extra}">
	{#if title || action}
		<div
			class="flex items-start justify-between gap-3 {flush
				? 'border-b border-gray-100 px-5 py-4 dark:border-gray-700/60'
				: 'mb-4'}"
		>
			<div class="min-w-0">
				{#if title}
					<h2
						class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100"
					>
						{#if icon}<IconifyIcon {icon} class="h-4 w-4 text-gray-400" />{/if}
						{title}
					</h2>
				{/if}
				{#if description}
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>
				{/if}
			</div>
			{#if action}<div class="flex flex-shrink-0 items-center gap-2">{@render action()}</div>{/if}
		</div>
	{/if}
	{@render children()}
</section>
