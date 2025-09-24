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
		class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-800 focus:ring-amber-500 transition-colors px-4 py-2 rounded-md
		{variant === 'clear'
			? 'text-gray-600 dark:text-gray-300 hover:text-orange-600 dark:hover:text-orange-400'
			: 'bg-amber-700 dark:bg-amber-600 text-white hover:bg-amber-600 dark:hover:bg-amber-500'} 
		{disabled ? 'opacity-50' : ''} 
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
		class="inline-flex items-center text-sm font-medium focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-800 focus:ring-amber-500 transition-colors px-4 py-2 rounded-md
		{variant === 'clear'
			? 'text-gray-600 dark:text-gray-300 hover:text-orange-600 dark:hover:text-orange-400'
			: 'bg-amber-700 dark:bg-amber-600 text-white hover:bg-amber-600 dark:hover:bg-amber-500'} 
		{disabled ? 'opacity-50' : ''} 
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
