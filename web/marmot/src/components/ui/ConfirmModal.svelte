<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { focusTrap } from '$lib/actions/focusTrap';
	import { BTN_DANGER, BTN_PRIMARY, BTN_SECONDARY } from '$lib/ui/classes';

	// A yes or no question on the same white dialog as Modal, with an optional checkbox for a follow-on choice such as also deleting what the thing created
	interface Props {
		show: boolean;
		title: string;
		message: string;
		confirmText?: string;
		cancelText?: string;
		variant?: 'danger' | 'warning' | 'info';
		checkboxLabel?: string;
		checkboxChecked?: boolean;
		onConfirm: (checkboxValue?: boolean) => void;
		onCancel?: () => void;
	}

	let {
		show = $bindable(),
		title,
		message,
		confirmText = m.common_confirm(),
		cancelText = m.common_cancel(),
		variant = 'danger',
		checkboxLabel,
		checkboxChecked = $bindable(false),
		onConfirm,
		onCancel
	}: Props = $props();

	const id = $props.id();

	function handleCancel() {
		if (onCancel) {
			onCancel();
		} else {
			show = false;
		}
	}

	const VARIANTS = {
		danger: {
			icon: 'material-symbols:warning-outline',
			iconBox: 'bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-400',
			button: BTN_DANGER
		},
		warning: {
			icon: 'material-symbols:info-outline',
			iconBox: 'bg-amber-50 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400',
			button:
				'inline-flex items-center justify-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-amber-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-500/40 disabled:cursor-not-allowed disabled:opacity-50'
		},
		info: {
			icon: 'material-symbols:info-outline',
			iconBox:
				'bg-earthy-blue-50 text-earthy-blue-700 dark:bg-earthy-blue-900/30 dark:text-earthy-blue-300',
			button: BTN_PRIMARY
		}
	};

	let v = $derived(VARIANTS[variant]);
</script>

<svelte:window onkeydown={(e) => show && e.key === 'Escape' && handleCancel()} />

{#if show}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto px-4 pt-[12vh]">
		<button
			type="button"
			class="fixed inset-0 cursor-default bg-black/30"
			onclick={handleCancel}
			aria-label={m.common_close()}
			tabindex="-1"
			transition:fade={{ duration: 150 }}
		></button>
		<div
			class="relative w-full max-w-md overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-800"
			role="dialog"
			aria-modal="true"
			aria-labelledby="{id}-title"
			transition:scale={{ start: 0.97, duration: 150 }}
			use:focusTrap
		>
			<div class="flex items-start gap-4 px-6 pb-5 pt-5">
				<span
					class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full {v.iconBox}"
				>
					<IconifyIcon icon={v.icon} class="h-5 w-5" />
				</span>
				<div class="min-w-0 flex-1">
					<h2
						id="{id}-title"
						class="text-lg font-semibold tracking-tight text-gray-900 dark:text-gray-100"
					>
						{title}
					</h2>
					<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{message}</p>
					{#if checkboxLabel}
						<label
							class="mt-4 flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5 dark:border-gray-700"
						>
							<input
								type="checkbox"
								bind:checked={checkboxChecked}
								class="h-4 w-4 cursor-pointer rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600 dark:bg-gray-900"
							/>
							<span class="text-sm text-gray-700 dark:text-gray-300">{checkboxLabel}</span>
						</label>
					{/if}
				</div>
			</div>

			<div
				class="flex items-center justify-end gap-2 border-t border-gray-100 bg-gray-50 px-6 py-3 dark:border-gray-700 dark:bg-gray-900/40"
			>
				<button type="button" class={BTN_SECONDARY} onclick={handleCancel}>{cancelText}</button>
				<button type="button" class={v.button} onclick={() => onConfirm(checkboxChecked)}>
					{confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
