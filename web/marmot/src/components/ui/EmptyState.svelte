<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconifyIcon from '@iconify/svelte';

	// What a list shows when it has nothing in it: what would be here, and the one action that fills it.
	let {
		icon = 'material-symbols:inbox-outline',
		title,
		description,
		dashed = true,
		action
	}: {
		icon?: string;
		title: string;
		description?: string;
		dashed?: boolean;
		action?: Snippet;
	} = $props();
</script>

<div
	class="flex flex-col items-center rounded-xl px-6 py-10 text-center {dashed
		? 'border border-dashed border-gray-300 dark:border-gray-700'
		: ''}"
>
	<IconifyIcon {icon} class="h-8 w-8 text-gray-300 dark:text-gray-600" />
	<p class="mt-3 text-sm font-medium text-gray-900 dark:text-gray-100">{title}</p>
	{#if description}
		<p class="mt-1 max-w-md text-sm text-gray-500 dark:text-gray-400">{description}</p>
	{/if}
	{#if action}<div class="mt-4">{@render action()}</div>{/if}
</div>
