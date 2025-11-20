<script lang="ts">
	import Icon from '@iconify/svelte';

	let {
		variant = 'filled',
		icon = null,
		text = '',
		href = null,
		target = null,
		loading = false,
		disabled = false,
		class: className = '',
		click = (event: MouseEvent) => {}
	} = $props<{
		variant?: 'clear' | 'filled';
		icon?: string | null;
		text?: string;
		href?: string | null;
		target?: string | null;
		loading?: boolean;
		disabled?: boolean;
		class?: string;
		click?: (event: MouseEvent) => void;
	}>();

	function handleClick(event: MouseEvent) {
		if (loading || disabled) {
			event.preventDefault();
			return;
		}
		click(event);
	}
</script>

{#if href}
	<a
		{href}
		{target}
		class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-800 focus:ring-earthy-terracotta-600 transition-colors px-4 py-2 rounded-lg shadow-sm
		{variant === 'clear'
			? 'text-gray-900 dark:text-gray-100 bg-gray-100 dark:bg-gray-700 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/20 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-500'
			: 'bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-800 hover:shadow-md'}
		{disabled && variant === 'clear' ? 'opacity-50' : ''}
		{className}"
		class:cursor-not-allowed={loading || disabled}
		onclick={handleClick}
	>
		{#if icon}
			<span class="w-4 h-4 mr-1.5">
				<Icon icon={`${icon}`} />
			</span>
		{/if}
		{#if text}
			<span>{text}</span>
		{/if}
		<slot />
	</a>
{:else}
	<button
		class="inline-flex items-center text-sm font-medium focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-800 focus:ring-earthy-terracotta-600 transition-colors px-4 py-2 rounded-lg shadow-sm
		{variant === 'clear'
			? 'text-gray-900 dark:text-gray-100 bg-gray-100 dark:bg-gray-700 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/20 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-500'
			: 'bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-800 hover:shadow-md'}
		{disabled && variant === 'clear' ? 'opacity-50' : ''}
		{className}"
		class:cursor-not-allowed={loading || disabled}
		onclick={handleClick}
		{disabled}
	>
		{#if loading}
			<span class="w-4 h-4 mr-1.5">
				<Icon icon="bi:arrow-clockwise" class="animate-spin" />
			</span>
		{:else if icon}
			<span class="w-4 h-4 mr-1.5">
				<Icon icon={`${icon}`} />
			</span>
		{/if}
		{#if text}
			<span>{text}</span>
		{/if}
		<slot />
	</button>
{/if}
