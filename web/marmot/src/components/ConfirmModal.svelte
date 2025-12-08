<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Button from './Button.svelte';

	interface Props {
		show: boolean;
		title: string;
		message: string;
		confirmText?: string;
		cancelText?: string;
		variant?: 'danger' | 'warning' | 'info';
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		show = $bindable(),
		title,
		message,
		confirmText = 'Confirm',
		cancelText = 'Cancel',
		variant = 'danger',
		onConfirm,
		onCancel
	}: Props = $props();

	let variantColors = $derived(
		variant === 'danger'
			? {
					icon: 'material-symbols:warning',
					iconBg: 'bg-red-100 dark:bg-red-900/20',
					iconColor: 'text-red-600 dark:text-red-400',
					buttonClass: 'bg-red-600 hover:bg-red-700 text-white'
				}
			: variant === 'warning'
				? {
						icon: 'material-symbols:info',
						iconBg: 'bg-amber-100 dark:bg-amber-900/20',
						iconColor: 'text-amber-600 dark:text-amber-400',
						buttonClass: 'bg-amber-600 hover:bg-amber-700 text-white'
					}
				: {
						icon: 'material-symbols:info',
						iconBg: 'bg-blue-100 dark:bg-blue-900/20',
						iconColor: 'text-blue-600 dark:text-blue-400',
						buttonClass: 'bg-blue-600 hover:bg-blue-700 text-white'
					}
	);
</script>

{#if show}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 p-4"
		onclick={onCancel}
		onkeydown={(e) => e.key === 'Escape' && onCancel()}
		role="button"
		tabindex="-1"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full overflow-hidden"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="p-6">
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0 {variantColors.iconBg} rounded-full p-3">
						<IconifyIcon icon={variantColors.icon} class="h-6 w-6 {variantColors.iconColor}" />
					</div>
					<div class="flex-1 min-w-0">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
							{title}
						</h3>
						<p class="text-sm text-gray-600 dark:text-gray-400">
							{message}
						</p>
					</div>
				</div>
			</div>

			<div
				class="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700"
			>
				<Button variant="clear" click={onCancel} text={cancelText} />
				<button
					onclick={onConfirm}
					class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium transition-colors {variantColors.buttonClass}"
				>
					{confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
