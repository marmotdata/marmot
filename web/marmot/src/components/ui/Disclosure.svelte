<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconifyIcon from '@iconify/svelte';

	// A flat collapsible row: a title, a muted hint of what is inside and a chevron, separated from its neighbours by hairlines rather than boxed
	interface Props {
		title: string;
		hint?: string;
		open?: boolean;
		children: Snippet;
	}

	let { title, hint, open = $bindable(false), children }: Props = $props();

	const contentId = $props.id();
</script>

<div>
	<button
		type="button"
		onclick={() => (open = !open)}
		aria-expanded={open}
		aria-controls={contentId}
		class="group flex w-full items-center gap-3 py-3 text-left"
	>
		<span
			class="flex-shrink-0 text-sm font-medium text-gray-800 group-hover:text-gray-950 dark:text-gray-200 dark:group-hover:text-white"
			>{title}</span
		>
		{#if hint && !open}
			<span class="min-w-0 truncate text-xs text-gray-400 dark:text-gray-500">{hint}</span>
		{/if}
		<IconifyIcon
			icon="material-symbols:expand-more"
			class="ml-auto h-5 w-5 flex-shrink-0 text-gray-400 transition-transform group-hover:text-gray-600 {open
				? 'rotate-180'
				: ''}"
		/>
	</button>
	{#if open}
		<div id={contentId} class="pb-5 pt-1">
			{@render children()}
		</div>
	{/if}
</div>
