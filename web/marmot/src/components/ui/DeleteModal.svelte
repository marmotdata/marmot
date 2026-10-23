<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { focusTrap } from '$lib/actions/focusTrap';
	import { BTN_DANGER, BTN_SECONDARY, INPUT } from '$lib/ui/classes';

	// The confirmation every destructive action goes through, on the same white dialog as Modal, optionally asking for the resource name to be typed before the delete button unlocks
	interface Props {
		show?: boolean;
		title: string;
		message: string;
		confirmText?: string;
		resourceName?: string;
		requireConfirmation?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		show = $bindable(false),
		title,
		message,
		confirmText = m.common_delete(),
		resourceName = '',
		requireConfirmation = false,
		onConfirm,
		onCancel
	}: Props = $props();

	const id = $props.id();
	let inputText = $state('');
	let isConfirmEnabled = $derived(
		!requireConfirmation || (!!resourceName && inputText === resourceName)
	);

	$effect(() => {
		if (!show) inputText = '';
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && isConfirmEnabled) {
			event.preventDefault();
			onConfirm();
		}
	}
</script>

<svelte:window onkeydown={(e) => show && e.key === 'Escape' && onCancel()} />

{#if show}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto px-4 pt-[12vh]">
		<button
			type="button"
			class="fixed inset-0 cursor-default bg-black/30"
			onclick={onCancel}
			aria-label={m.common_close()}
			tabindex="-1"
			transition:fade={{ duration: 150 }}
		></button>
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="{id}-title"
			class="relative w-full max-w-lg overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-800"
			transition:scale={{ start: 0.97, duration: 150 }}
			use:focusTrap
		>
			<div class="flex items-start gap-4 px-6 pb-5 pt-5">
				<span
					class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-red-50 dark:bg-red-950/40"
				>
					<IconifyIcon
						icon="material-symbols:delete-outline"
						class="h-5 w-5 text-red-600 dark:text-red-400"
					/>
				</span>
				<div class="min-w-0 flex-1">
					<h2
						id="{id}-title"
						class="text-lg font-semibold tracking-tight text-gray-900 dark:text-gray-100"
					>
						{title}
					</h2>
					<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{message}</p>
					{#if requireConfirmation}
						<label for="{id}-confirm" class="mt-4 block text-sm text-gray-700 dark:text-gray-300">
							{m.ui_deletemodal_type_to_confirm({ name: resourceName })}
						</label>
						<input
							type="text"
							id="{id}-confirm"
							bind:value={inputText}
							autocomplete="off"
							class="{INPUT} mt-1.5 font-mono"
							onkeydown={handleKeydown}
						/>
					{/if}
				</div>
			</div>
			<div
				class="flex items-center justify-end gap-2 border-t border-gray-100 bg-gray-50 px-6 py-3 dark:border-gray-700 dark:bg-gray-900/40"
			>
				<button type="button" class={BTN_SECONDARY} onclick={onCancel}>
					{m.common_cancel()}
				</button>
				<button type="button" class={BTN_DANGER} disabled={!isConfirmEnabled} onclick={onConfirm}>
					{confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
