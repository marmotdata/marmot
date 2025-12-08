<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

	let {
		tags = $bindable([]),
		disabled = false,
		placeholder = 'Type a tag and press Enter...'
	}: { tags: string[]; disabled?: boolean; placeholder?: string } = $props();

	let tagInput = $state('');

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && tagInput.trim()) {
			event.preventDefault();
			if (!tags.includes(tagInput.trim())) {
				tags = [...tags, tagInput.trim()];
			}
			tagInput = '';
		}
	}

	function removeTag(tag: string) {
		tags = tags.filter((t) => t !== tag);
	}
</script>

<div class="space-y-3">
	<input
		type="text"
		bind:value={tagInput}
		onkeydown={handleKeydown}
		{disabled}
		{placeholder}
		class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all"
	/>
	{#if tags.length > 0}
		<div class="flex flex-wrap gap-2">
			{#each tags as tag}
				<span
					class="inline-flex items-center gap-2 px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg"
				>
					{tag}
					<button
						type="button"
						onclick={() => removeTag(tag)}
						{disabled}
						class="text-gray-500 hover:text-red-600 dark:hover:text-red-400 transition-colors disabled:opacity-50"
					>
						<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
					</button>
				</span>
			{/each}
		</div>
	{/if}
</div>
