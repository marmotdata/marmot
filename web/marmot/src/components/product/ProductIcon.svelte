<script lang="ts">
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';

	// The tile a data product is recognised by in lists, the blade and the page header: its uploaded icon, or a neutral glyph until it has one.
	let {
		iconUrl = null,
		size = 'md'
	}: {
		iconUrl?: string | null;
		size?: 'sm' | 'md' | 'lg';
	} = $props();

	const px = { sm: 32, md: 40, lg: 48 } as const;
	const glyph = { sm: 16, md: 20, lg: 24 } as const;
</script>

{#snippet fallback()}
	<mm-icon name="inventory-2" size={glyph[size]}></mm-icon>
{/snippet}

<span class="tile" style:--tile={`${px[size]}px`} aria-hidden="true">
	{#if iconUrl}
		<AuthenticatedImage src={iconUrl} alt="" class="tile-img" {fallback} />
	{:else}
		{@render fallback()}
	{/if}
</span>

<style>
	.tile {
		display: grid;
		place-items: center;
		flex: none;
		width: var(--tile);
		height: var(--tile);
		overflow: hidden;
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-md);
		background: var(--mm-surface);
		color: var(--mm-text-2);
	}
	.tile :global(.tile-img) {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
</style>
