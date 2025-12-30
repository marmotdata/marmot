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
		checkboxLabel?: string;
		checkboxChecked?: boolean;
		onConfirm: (checkboxValue?: boolean) => void;
		onCancel?: () => void;
	}

	let {
		show = $bindable(),
		title,
		message,
		confirmText = 'Confirm',
		cancelText = 'Cancel',
		variant = 'danger',
		checkboxLabel,
		checkboxChecked = $bindable(false),
		onConfirm,
		onCancel
	}: Props = $props();

	function handleCancel() {
		if (onCancel) {
			onCancel();
		} else {
			show = false;
		}
	}

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
		onclick={handleCancel}
		onkeydown={(e) => e.key === 'Escape' && handleCancel()}
		role="presentation"
		aria-hidden="true"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full overflow-hidden"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="modal-title"
		>
			<div class="p-6">
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0 {variantColors.iconBg} rounded-full p-3">
						<IconifyIcon icon={variantColors.icon} class="h-6 w-6 {variantColors.iconColor}" />
					</div>
					<div class="flex-1 min-w-0">
						<h3
							id="modal-title"
							class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2"
						>
							{title}
						</h3>
						<p class="text-sm text-gray-600 dark:text-gray-400">
							{message}
						</p>
					</div>
				</div>
				{#if checkboxLabel}
					<div
						class="mt-4 mx-6 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg"
					>
						<label class="flex items-center gap-3 cursor-pointer group">
							<input
								type="checkbox"
								bind:checked={checkboxChecked}
								class="w-5 h-5 text-amber-600 bg-white dark:bg-gray-700 border-amber-300 dark:border-amber-700 rounded focus:ring-2 focus:ring-amber-500 dark:focus:ring-amber-600 cursor-pointer transition-all"
							/>
							<span
								class="text-sm font-medium text-amber-900 dark:text-amber-100 group-hover:text-amber-800 dark:group-hover:text-amber-50 transition-colors"
							>
								{checkboxLabel}
							</span>
						</label>
					</div>
				{/if}
			</div>

			<div class="border-t border-gray-200 dark:border-gray-700">
				<div class="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50 dark:bg-gray-900/50">
					<Button variant="clear" click={handleCancel} text={cancelText} />
					<button
						onclick={() => onConfirm(checkboxChecked)}
						class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium transition-colors {variantColors.buttonClass}"
					>
						{confirmText}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
