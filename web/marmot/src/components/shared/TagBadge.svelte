<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

	interface Props {
		name: string;
		title?: string;
		onRemove?: () => void;
		disabled?: boolean;
		class?: string;
	}

	let { name, title, onRemove, disabled = false, class: extraClass = '' }: Props = $props();
</script>

<span
	class="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 {extraClass}"
	{title}
>
	{name}
	{#if onRemove}
		<button
			onclick={onRemove}
			{disabled}
			class="ml-0.5 hover:text-red-600 dark:hover:text-red-400 disabled:opacity-50 transition-colors"
			aria-label="Remove {name}"
		>
			<IconifyIcon icon="material-symbols:close-small-rounded" class="w-3.5 h-3.5" />
		</button>
	{/if}
</span>
