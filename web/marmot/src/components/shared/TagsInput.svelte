<script lang="ts">
	// A free-text tag field for create forms: Enter adds the typed tag as an mm-tag chip below.
	import { INPUT } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let {
		tags = $bindable([]),
		disabled = false,
		placeholder = m.tags_input_placeholder()
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

<div class="mm-stack" style="--mm-gap: var(--space-2)">
	<input
		type="text"
		bind:value={tagInput}
		onkeydown={handleKeydown}
		{disabled}
		{placeholder}
		aria-label={m.tags_add_tags_label()}
		class="{INPUT} disabled:opacity-50"
	/>
	{#if tags.length > 0}
		<div class="mm-cluster" style="--mm-gap: var(--space-1)">
			{#each tags as tag (tag)}
				<mm-tag
					removable={!disabled}
					onmm-remove={(e: Event) => {
						e.preventDefault();
						removeTag(tag);
					}}>{tag}</mm-tag
				>
			{/each}
		</div>
	{/if}
</div>
