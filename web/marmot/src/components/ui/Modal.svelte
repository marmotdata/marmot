<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fade, scale } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { focusTrap } from '$lib/actions/focusTrap';

	// The shared shell for short, focused tasks: a centred dialog over a dimmed page, closed with Esc or the backdrop, with the same title row, discard guard and footer as Blade so a form can move between the two
	interface Props {
		title: string;
		subtitle?: string;
		onClose: () => void;
		width?: 'md' | 'lg' | 'xl';
		dirty?: boolean;
		children?: Snippet;
		footer?: Snippet<[close: () => void]>;
	}

	let { title, subtitle, onClose, width = 'lg', dirty = false, children, footer }: Props = $props();

	const WIDTHS = { md: 'max-w-md', lg: 'max-w-lg', xl: 'max-w-xl' };
	const titleId = $props.id();

	let confirmingDiscard = $state(false);

	function requestClose() {
		if (dirty && !confirmingDiscard) {
			confirmingDiscard = true;
			return;
		}
		confirmingDiscard = false;
		onClose();
	}
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && requestClose()} />

<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto px-4 pt-[12vh]">
	<button
		type="button"
		class="fixed inset-0 cursor-default bg-black/30"
		onclick={requestClose}
		aria-label={m.common_close()}
		tabindex="-1"
		transition:fade={{ duration: 150 }}
	></button>
	<div
		class="relative w-full {WIDTHS[
			width
		]} overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-800"
		role="dialog"
		aria-modal="true"
		aria-labelledby="{titleId}-title"
		transition:scale={{ start: 0.97, duration: 150 }}
		use:focusTrap
	>
		<div class="flex items-start justify-between gap-4 px-6 pt-5">
			<div class="min-w-0">
				<h2 id="{titleId}-title" class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{title}
				</h2>
				{#if subtitle}
					<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
				{/if}
			</div>
			<button
				type="button"
				onclick={requestClose}
				class="-mr-2 rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-700 dark:hover:text-gray-200"
				aria-label={m.common_close()}
			>
				<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
			</button>
		</div>

		{#if children}
			<div class="px-6 py-5">
				{@render children()}
			</div>
		{:else}
			<div class="pb-5"></div>
		{/if}

		{#if confirmingDiscard}
			<div
				class="flex items-center justify-between gap-3 border-t border-amber-200 bg-amber-50 px-6 py-3 dark:border-amber-900/60 dark:bg-amber-950/30"
				role="alert"
			>
				<span class="text-sm font-medium text-amber-900 dark:text-amber-200"
					>{m.common_discard_prompt()}</span
				>
				<div class="flex gap-2">
					<button
						type="button"
						onclick={() => (confirmingDiscard = false)}
						class="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-white dark:text-gray-200 dark:hover:bg-gray-800"
						>{m.common_keep_editing()}</button
					>
					<button
						type="button"
						onclick={requestClose}
						class="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-red-500"
						>{m.common_discard()}</button
					>
				</div>
			</div>
		{/if}

		{#if footer}
			<div
				class="flex items-center justify-end gap-2 border-t border-gray-100 bg-gray-50 px-6 py-3 dark:border-gray-700 dark:bg-gray-900/40"
			>
				{@render footer(requestClose)}
			</div>
		{/if}
	</div>
</div>
