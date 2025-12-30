<script lang="ts">
	import { toasts, type Toast } from '$lib/stores/toast';
	import IconifyIcon from '@iconify/svelte';

	const variantStyles: Record<
		Toast['variant'],
		{ icon: string; bg: string; border: string; iconColor: string; textColor: string }
	> = {
		success: {
			icon: 'material-symbols:check-circle',
			bg: 'bg-green-50 dark:bg-green-900/20',
			border: 'border-green-200 dark:border-green-800/50',
			iconColor: 'text-green-600 dark:text-green-400',
			textColor: 'text-green-800 dark:text-green-200'
		},
		error: {
			icon: 'material-symbols:error',
			bg: 'bg-red-50 dark:bg-red-900/20',
			border: 'border-red-200 dark:border-red-800/50',
			iconColor: 'text-red-600 dark:text-red-400',
			textColor: 'text-red-800 dark:text-red-200'
		},
		warning: {
			icon: 'material-symbols:warning',
			bg: 'bg-amber-50 dark:bg-amber-900/20',
			border: 'border-amber-200 dark:border-amber-800/50',
			iconColor: 'text-amber-600 dark:text-amber-400',
			textColor: 'text-amber-800 dark:text-amber-200'
		},
		info: {
			icon: 'material-symbols:info',
			bg: 'bg-blue-50 dark:bg-blue-900/20',
			border: 'border-blue-200 dark:border-blue-800/50',
			iconColor: 'text-blue-600 dark:text-blue-400',
			textColor: 'text-blue-800 dark:text-blue-200'
		}
	};
</script>

{#if $toasts.length > 0}
	<div class="fixed top-4 right-4 z-50 flex flex-col gap-2">
		{#each $toasts as toast (toast.id)}
			<div
				class="animate-in slide-in-from-top-2 fade-in duration-200 flex items-center gap-3 min-w-[300px] max-w-md px-4 py-3 {variantStyles[
					toast.variant
				].bg} border {variantStyles[toast.variant].border} rounded-lg shadow-lg"
				role="alert"
			>
				<IconifyIcon
					icon={variantStyles[toast.variant].icon}
					class="h-5 w-5 {variantStyles[toast.variant].iconColor} flex-shrink-0"
				/>
				<p class="flex-1 text-sm font-medium {variantStyles[toast.variant].textColor}">
					{toast.message}
				</p>
				<button
					onclick={() => toasts.remove(toast.id)}
					class="flex-shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors"
					aria-label="Dismiss notification"
				>
					<IconifyIcon icon="material-symbols:close" class="h-5 w-5" />
				</button>
			</div>
		{/each}
	</div>
{/if}
