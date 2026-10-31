<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import type { Asset } from '$lib/assets/types';
	import { assetCaption, assetProvider, searchAssets } from '$lib/dataproducts/assets';

	// Search results each carry their own Add button that acts at once; what is already chosen is listed elsewhere, never mixed into the results.
	let {
		exclude = [],
		onAdd,
		onClose
	}: {
		exclude?: string[];
		onAdd: (asset: Asset) => void | Promise<void>;
		onClose?: () => void;
	} = $props();

	let query = $state('');
	let results = $state<Asset[]>([]);
	let searching = $state(false);
	let adding = $state<string | null>(null);
	let searched = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let token = 0;

	let shown = $derived(results.filter((a) => !exclude.includes(a.id)));

	function handleInput(event: Event) {
		query = (event.target as HTMLInputElement).value;
		clearTimeout(timer);
		if (!query.trim()) {
			results = [];
			searched = '';
			return;
		}
		timer = setTimeout(run, 300);
	}

	async function run() {
		const q = query.trim();
		const mine = ++token;
		searching = true;
		const found = await searchAssets(q, 10);
		if (mine !== token) return;
		results = found;
		searched = q;
		searching = false;
	}

	async function add(asset: Asset) {
		adding = asset.id;
		try {
			await onAdd(asset);
		} finally {
			adding = null;
		}
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-3)">
	<div class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
		<!-- svelte-ignore a11y_autofocus -->
		<mm-search-field
			class="grow"
			label={m.products_picker_label()}
			hide-label
			placeholder={m.products_picker_placeholder()}
			value={query}
			autofocus
			oninput={handleInput}
			onmm-search={(e: Event) => e.preventDefault()}
		></mm-search-field>
		{#if onClose}
			<mm-button variant="ghost" onclick={onClose}>{m.products_picker_done()}</mm-button>
		{/if}
	</div>
	{#if searching && shown.length === 0}
		<mm-skeleton loading count={3}></mm-skeleton>
	{:else if shown.length > 0}
		<mm-list aria-label={m.products_picker_results()} style="--mm-list-inset: 0">
			{#each shown as asset (asset.id)}
				<mm-list-item secondary={assetCaption(asset)}>
					<mm-provider-logo slot="start" provider={assetProvider(asset)} size={20} decorative
					></mm-provider-logo>
					{asset.name}
					<mm-button
						slot="end"
						size="sm"
						icon="add"
						loading={adding === asset.id}
						label={m.products_picker_add_named({ name: asset.name })}
						onclick={() => add(asset)}>{m.common_add()}</mm-button
					>
				</mm-list-item>
			{/each}
		</mm-list>
	{:else if searched}
		<mm-text variant="dense" tone="secondary" role="status">{m.products_picker_none()}</mm-text>
	{/if}
</div>

<style>
	.grow {
		flex: 1;
		min-width: 0;
		max-width: 28rem;
	}
</style>
