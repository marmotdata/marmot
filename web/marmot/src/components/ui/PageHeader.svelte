<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconifyIcon from '@iconify/svelte';

	// The header every page opens with: title, one line saying what the page is for, and its actions on the right.
	let {
		title,
		description,
		backHref,
		backLabel,
		eyebrow,
		actions,
		meta
	}: {
		title: string;
		description?: string;
		backHref?: string;
		backLabel?: string;
		eyebrow?: string;
		actions?: Snippet;
		meta?: Snippet;
	} = $props();
</script>

<header class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div class="min-w-0">
		{#if backHref}
			<!-- Callers pass a href they already built with resolve() -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				href={backHref}
				class="mb-2 inline-flex items-center gap-1 text-sm text-gray-500 transition hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
			>
				<IconifyIcon icon="material-symbols:arrow-back" class="h-4 w-4" />
				{backLabel ?? ''}
			</a>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/if}
		{#if eyebrow}
			<p class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
				{eyebrow}
			</p>
		{/if}
		<h1 class="truncate text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
			{title}
		</h1>
		{#if description}
			<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{description}</p>
		{/if}
		{#if meta}
			<div class="mt-2 flex flex-wrap items-center gap-2">{@render meta()}</div>
		{/if}
	</div>
	{#if actions}
		<div class="flex flex-shrink-0 flex-wrap items-center gap-2">{@render actions()}</div>
	{/if}
</header>
