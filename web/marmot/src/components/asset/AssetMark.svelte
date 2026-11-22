<script lang="ts">
	import { kindOfType, knowledgeKinds } from '#lib/knowledge/kinds.ts';

	// The one mark for an asset in a list, header or blade. A knowledge entry shows its kind's icon, since every entry's provider is Marmot and the kind says more; any other asset shows its provider's logo, falling back to the type when it has several providers or none, as lists did before.
	let {
		asset,
		size = 20
	}: {
		asset: { type?: string; providers?: string[]; provider?: string } | null | undefined;
		size?: number;
	} = $props();

	let kind = $derived(kindOfType(asset?.type, $knowledgeKinds));
	let provider = $derived(
		(asset?.provider || asset?.providers?.[0] || asset?.type || '').toLowerCase()
	);
</script>

{#if kind}
	<mm-icon name={kind.icon} {size} aria-hidden="true"></mm-icon>
{:else if provider}
	<mm-provider-logo {provider} {size} decorative></mm-provider-logo>
{:else}
	<mm-icon name="database" {size} aria-hidden="true"></mm-icon>
{/if}
