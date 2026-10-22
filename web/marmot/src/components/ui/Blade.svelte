<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { focusTrap } from '$lib/actions/focusTrap';

	// The shared shell for side blades: a dimmed backdrop and a panel sliding in from the right, or an inline column when staticPlacement is set, with the title row, scrolling body and optional footer every blade repeats
	interface Props {
		title: string;
		subtitle?: string;
		onClose: () => void;
		width?: 'xl' | '2xl' | '3xl';
		staticPlacement?: boolean;
		collapsed?: boolean;
		onToggleCollapse?: () => void;
		bodyClass?: string;
		// When set, closing asks before throwing away what was typed
		dirty?: boolean;
		actions?: Snippet;
		children: Snippet;
		// The footer receives the guarded close, so a Cancel button asks before discarding just like the close button does
		footer?: Snippet<[close: () => void]>;
	}

	let {
		title,
		subtitle,
		onClose,
		width = '2xl',
		staticPlacement = false,
		collapsed = false,
		onToggleCollapse,
		bodyClass,
		dirty = false,
		actions,
		children,
		footer
	}: Props = $props();

	const WIDTHS = { xl: 'max-w-xl', '2xl': 'max-w-2xl', '3xl': 'max-w-3xl' };
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

<svelte:window onkeydown={(e) => !staticPlacement && e.key === 'Escape' && requestClose()} />

{#if !staticPlacement}
	<button
		type="button"
		class="fixed inset-0 z-40 cursor-default bg-black/30"
		onclick={requestClose}
		aria-label={m.common_close()}
		tabindex="-1"
		transition:fade={{ duration: 200 }}
	></button>
{/if}

<div
	class={staticPlacement
		? 'flex h-full w-full bg-earthy-brown-50 dark:bg-gray-900'
		: `fixed right-0 top-0 z-50 flex h-full w-full ${WIDTHS[width]} flex-col bg-earthy-brown-50 shadow-lg dark:bg-gray-900 dark:shadow-2xl`}
	role={staticPlacement ? undefined : 'dialog'}
	aria-modal={staticPlacement ? undefined : true}
	aria-labelledby="{titleId}-title"
	transition:fly={{ x: staticPlacement ? 0 : 400, duration: staticPlacement ? 0 : 200 }}
	use:focusTrap={!staticPlacement}
>
	{#if staticPlacement && onToggleCollapse}
		<button
			type="button"
			onclick={onToggleCollapse}
			class="flex w-8 flex-shrink-0 items-center justify-center transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
			aria-label={collapsed
				? m.asset_blade_expand_sidebar_aria()
				: m.asset_blade_collapse_sidebar_aria()}
		>
			<IconifyIcon
				icon="material-symbols:chevron-right"
				class="h-5 w-5 text-gray-600 transition-transform dark:text-gray-400 {collapsed
					? 'rotate-180'
					: ''}"
			/>
		</button>
	{/if}

	{#if !collapsed}
		<div class="flex min-h-0 min-w-0 flex-1 flex-col">
			<div
				class="flex flex-none items-center justify-between gap-4 border-b border-gray-200 bg-earthy-brown-50 px-6 py-4 dark:border-gray-700 dark:bg-gray-900"
			>
				<div class="min-w-0">
					<h2
						id="{titleId}-title"
						class="truncate text-2xl font-bold text-gray-900 dark:text-gray-100"
					>
						{title}
					</h2>
					{#if subtitle}
						<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
					{/if}
				</div>
				{#if !staticPlacement}
					<div class="flex flex-shrink-0 items-center gap-3">
						{@render actions?.()}
						<button
							type="button"
							onclick={requestClose}
							class="rounded-lg p-2 text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-100"
							aria-label={m.common_close()}
						>
							<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
						</button>
					</div>
				{/if}
			</div>

			<div
				class="min-h-0 flex-1 overflow-y-auto {bodyClass ??
					(staticPlacement ? 'py-6 pr-6' : 'p-6')}"
			>
				{@render children()}
			</div>

			{#if confirmingDiscard}
				<div
					class="flex flex-none items-center justify-between gap-3 border-t border-amber-200 bg-amber-50 px-6 py-3 dark:border-amber-900/60 dark:bg-amber-950/30"
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
					class="flex-none border-t border-gray-200 bg-earthy-brown-50 px-6 py-4 dark:border-gray-700 dark:bg-gray-900"
				>
					{@render footer(requestClose)}
				</div>
			{/if}
		</div>
	{/if}
</div>
