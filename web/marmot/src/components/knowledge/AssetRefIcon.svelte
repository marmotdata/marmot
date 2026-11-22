<script lang="ts">
	import { kindOfType, knowledgeKinds } from '#lib/knowledge/kinds.ts';
	import type { KnowledgeAssetRef } from '#lib/knowledge/api.ts';

	// The mark for an asset an entry links to: a knowledge entry's kind icon, otherwise its provider's logo, as Discover shows it.
	let { asset, size = 16 }: { asset: Pick<KnowledgeAssetRef, 'type' | 'provider'>; size?: number } =
		$props();

	let kind = $derived(kindOfType(asset.type, $knowledgeKinds));
</script>

{#if kind}
	<mm-icon name={kind.icon} {size} aria-hidden="true"></mm-icon>
{:else if asset.provider}
	<mm-provider-logo provider={asset.provider.toLowerCase()} {size} decorative></mm-provider-logo>
{:else}
	<mm-icon name="database" {size} aria-hidden="true"></mm-icon>
{/if}
