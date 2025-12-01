<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';

	interface Props {
		show: boolean;
		message: string;
		variant?: 'success' | 'error' | 'info';
		duration?: number;
		onClose: () => void;
	}

	let {
		show = $bindable(),
		message,
		variant = 'info',
		duration = 3000,
		onClose
	}: Props = $props();

	let variantStyles = $derived(
		variant === 'success'
			? {
					icon: 'material-symbols:check-circle',
					bg: 'bg-green-50 dark:bg-green-900/20',
					border: 'border-green-200 dark:border-green-800/50',
					iconColor: 'text-green-600 dark:text-green-400',
					textColor: 'text-green-800 dark:text-green-200'
				}
			: variant === 'error'
				? {
						icon: 'material-symbols:error',
						bg: 'bg-red-50 dark:bg-red-900/20',
						border: 'border-red-200 dark:border-red-800/50',
						iconColor: 'text-red-600 dark:text-red-400',
						textColor: 'text-red-800 dark:text-red-200'
					}
				: {
						icon: 'material-symbols:info',
						bg: 'bg-blue-50 dark:bg-blue-900/20',
						border: 'border-blue-200 dark:border-blue-800/50',
						iconColor: 'text-blue-600 dark:text-blue-400',
						textColor: 'text-blue-800 dark:text-blue-200'
					}
	);

	$effect(() => {
		if (show && duration > 0) {
			const timer = setTimeout(() => {
				show = false;
				onClose();
			}, duration);

			return () => clearTimeout(timer);
		}
	});
</script>

{#if show}
	<div class="fixed top-4 right-4 z-50 animate-in slide-in-from-top-2 fade-in duration-200">
		<div
			class="flex items-center gap-3 min-w-[300px] max-w-md px-4 py-3 {variantStyles.bg} border {variantStyles.border} rounded-lg shadow-lg"
		>
			<IconifyIcon icon={variantStyles.icon} class="h-5 w-5 {variantStyles.iconColor} flex-shrink-0" />
			<p class="flex-1 text-sm font-medium {variantStyles.textColor}">
				{message}
			</p>
			<button
				onclick={() => {
					show = false;
					onClose();
				}}
				class="flex-shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors"
			>
				<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
			</button>
		</div>
	</div>
{/if}
