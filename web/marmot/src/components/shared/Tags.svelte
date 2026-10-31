<script lang="ts">
	// Tags on an asset, product or term as mm-tag chips, saved straight to the endpoint when one is given or kept in the bound array otherwise.
	import { fetchApi } from '$lib/api';
	import { INPUT } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let {
		tags = $bindable([]),
		endpoint,
		id,
		canEdit = false,
		saving = $bindable(false)
	}: {
		tags: string[];
		endpoint?: string;
		id?: string;
		canEdit?: boolean;
		saving?: boolean;
	} = $props();

	let newTag = $state('');
	let showTagInput = $state(false);

	// Local-only mode: when no endpoint/id, just update the bound tags array directly
	const isLocalMode = $derived(!endpoint || !id);

	async function addTag() {
		if (!newTag.trim()) return;

		const tagToAdd = newTag.trim();

		// Local-only mode: just update the tags array directly
		if (isLocalMode) {
			if (!tags.includes(tagToAdd)) {
				tags = [...tags, tagToAdd];
			}
			newTag = '';
			showTagInput = false;
			return;
		}

		saving = true;
		try {
			const updatedTags = [...tags, tagToAdd];

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
				newTag = '';
				showTagInput = false;
			} else {
				console.error('Failed to add tag');
			}
		} catch (error) {
			console.error('Error adding tag:', error);
		} finally {
			saving = false;
		}
	}

	async function removeTag(tag: string) {
		// Local-only mode: just update the tags array directly
		if (isLocalMode) {
			tags = tags.filter((t) => t !== tag);
			return;
		}

		saving = true;
		try {
			const updatedTags = tags.filter((t) => t !== tag);

			const response = await fetchApi(`${endpoint}/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					tags: updatedTags
				})
			});

			if (response.ok) {
				tags = updatedTags;
			} else {
				console.error('Failed to remove tag');
			}
		} catch (error) {
			console.error('Error removing tag:', error);
		} finally {
			saving = false;
		}
	}

	function cancelAdding() {
		showTagInput = false;
		newTag = '';
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-2)">
	<div class="mm-cluster" style="--mm-gap: var(--space-1)">
		{#each tags as tag (tag)}
			<mm-tag
				removable={canEdit && !saving}
				onmm-remove={(e: Event) => {
					e.preventDefault();
					removeTag(tag);
				}}>{tag}</mm-tag
			>
		{/each}

		{#if showTagInput}
			<span class="inline-flex items-center gap-space-1">
				<input
					type="text"
					bind:value={newTag}
					onkeydown={(e) => {
						if (e.key === 'Enter') addTag();
						if (e.key === 'Escape') cancelAdding();
					}}
					placeholder={m.tags_new_tag_placeholder()}
					aria-label={m.tags_enter_new_tag_aria()}
					class="{INPUT} h-[var(--control-h-sm)] !w-32 py-0"
					autofocus
				/>
				<mm-button
					size="sm"
					variant="ghost"
					icon="check"
					label={m.tags_add_tag()}
					disabled={saving || !newTag.trim()}
					onclick={addTag}
				></mm-button>
				<mm-button
					size="sm"
					variant="ghost"
					icon="close"
					label={m.tags_cancel_adding_aria()}
					disabled={saving}
					onclick={cancelAdding}
				></mm-button>
			</span>
		{/if}

		{#if canEdit && !showTagInput}
			<mm-button
				size="sm"
				variant="ghost"
				icon="add"
				disabled={saving}
				onclick={() => (showTagInput = true)}>{m.tags_add_tag()}</mm-button
			>
		{/if}
	</div>

	{#if tags.length === 0 && !canEdit}
		<p class="text-caption text-mm-text-2">{m.tags_none()}</p>
	{/if}
</div>
