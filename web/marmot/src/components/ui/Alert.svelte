<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconifyIcon from '@iconify/svelte';

	// An inline message about the page or panel it sits in, in both themes.
	let {
		tone = 'error',
		title,
		children
	}: {
		tone?: 'error' | 'warning' | 'info' | 'success';
		title?: string;
		children?: Snippet;
	} = $props();

	const tones = {
		error: {
			box: 'border-red-200 bg-red-50 text-red-800 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300',
			icon: 'material-symbols:error-outline'
		},
		warning: {
			box: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300',
			icon: 'material-symbols:warning-outline'
		},
		info: {
			box: 'border-earthy-blue-200 bg-earthy-blue-50 text-earthy-blue-900 dark:border-earthy-blue-900/50 dark:bg-earthy-blue-900/20 dark:text-earthy-blue-200',
			icon: 'material-symbols:info-outline'
		},
		success: {
			box: 'border-earthy-green-200 bg-earthy-green-50 text-earthy-green-900 dark:border-earthy-green-900/50 dark:bg-earthy-green-900/20 dark:text-earthy-green-200',
			icon: 'material-symbols:check-circle-outline'
		}
	};
</script>

<div
	class="flex gap-3 rounded-xl border p-4 text-sm {tones[tone].box}"
	role={tone === 'error' ? 'alert' : 'status'}
>
	<IconifyIcon icon={tones[tone].icon} class="mt-0.5 h-4 w-4 flex-shrink-0" />
	<div class="min-w-0">
		{#if title}<p class="font-medium">{title}</p>{/if}
		{#if children}<div class={title ? 'mt-1' : ''}>{@render children()}</div>{/if}
	</div>
</div>
