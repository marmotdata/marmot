<script lang="ts" generics="T extends string">
	import IconifyIcon from '@iconify/svelte';

	// A small set of mutually exclusive views or ranges. The selected segment is inverted, which reads in both themes without a brand colour.
	let {
		options,
		value,
		onChange,
		size = 'md',
		label
	}: {
		options: { id: T; label: string; icon?: string }[];
		value: T;
		onChange: (id: T) => void;
		size?: 'sm' | 'md';
		label?: string;
	} = $props();
</script>

<div
	class="inline-flex self-start rounded-lg border border-gray-200 bg-white p-0.5 dark:border-gray-700 dark:bg-gray-800"
	role="tablist"
	aria-label={label}
>
	{#each options as option (option.id)}
		<button
			type="button"
			role="tab"
			aria-selected={value === option.id}
			onclick={() => onChange(option.id)}
			class="inline-flex items-center gap-2 rounded-md font-medium transition {size === 'sm'
				? 'px-3 py-1 text-xs'
				: 'px-3.5 py-1.5 text-sm'} {value === option.id
				? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
				: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
		>
			{#if option.icon}<IconifyIcon icon={option.icon} class="h-4 w-4" />{/if}
			{option.label}
		</button>
	{/each}
</div>
