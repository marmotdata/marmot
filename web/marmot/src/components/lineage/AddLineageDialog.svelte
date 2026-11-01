<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import type { Asset } from '$lib/assets/types';
	import { providerOf } from './graph';

	let {
		open = $bindable(false),
		direction,
		assetName,
		onAdd
	}: {
		open: boolean;
		direction: 'upstream' | 'downstream';
		/** Name of the asset the new edge attaches to. */
		assetName: string;
		/** Creates the edge to the picked asset's MRN; rejects with a message to show. */
		onAdd: (mrn: string) => Promise<void>;
	} = $props();

	let query = $state('');
	let results = $state<Asset[]>([]);
	let searching = $state(false);
	let adding = $state(false);
	let error = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0;

	$effect(() => {
		if (open) {
			query = '';
			results = [];
			error = '';
		}
	});

	async function search(q: string) {
		const mine = ++seq;
		if (!q.trim()) {
			results = [];
			searching = false;
			return;
		}
		searching = true;
		error = '';
		try {
			const res = await fetchApi(`/assets/search?q=${encodeURIComponent(q)}&limit=20`);
			if (!res.ok) throw new Error();
			const data = await res.json();
			if (mine === seq) results = data.assets ?? [];
		} catch {
			if (mine === seq) {
				error = m.lineage_search_error();
				results = [];
			}
		} finally {
			if (mine === seq) searching = false;
		}
	}

	function onInput(e: CustomEvent<{ value: string }>) {
		query = e.detail.value;
		clearTimeout(timer);
		timer = setTimeout(() => search(query), 250);
	}

	async function pick(asset: Asset) {
		if (adding) return;
		adding = true;
		error = '';
		try {
			await onAdd(asset.mrn);
			open = false;
		} catch (err) {
			error = err instanceof Error && err.message ? err.message : m.lineage_create_error();
		} finally {
			adding = false;
		}
	}
</script>

<mm-dialog
	{open}
	heading={direction === 'upstream'
		? m.lineage_add_upstream_heading()
		: m.lineage_add_downstream_heading()}
	description={direction === 'upstream'
		? m.lineage_add_upstream_description({ name: assetName })
		: m.lineage_add_downstream_description({ name: assetName })}
	onmm-request-close={(e: Event) => adding && e.preventDefault()}
	onmm-close={() => (open = false)}
>
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-search-field
			label={m.lineage_search_label()}
			placeholder={m.lineage_search_placeholder()}
			value={query}
			disabled={adding}
			autofocus
			onmm-input={onInput}
			onmm-search={(e: CustomEvent<{ value: string }>) => {
				e.preventDefault();
				clearTimeout(timer);
				search(e.detail.value);
			}}
		></mm-search-field>

		{#if error}
			<mm-callout tone="danger" compact>{error}</mm-callout>
		{/if}

		<div class="results" aria-busy={searching} tabindex="-1">
			{#if results.length}
				<mm-list dense>
					{#each results as asset (asset.id)}
						<mm-list-item
							interactive
							disabled={adding}
							secondary={asset.type}
							onmm-activate={() => pick(asset)}
						>
							{#if providerOf(asset)}
								<mm-provider-logo slot="start" provider={providerOf(asset)} size="20" decorative
								></mm-provider-logo>
							{/if}
							{asset.name}
						</mm-list-item>
					{/each}
				</mm-list>
			{:else if query.trim() && !searching && !error}
				<mm-text variant="dense" tone="secondary" block>{m.lineage_no_assets_found()}</mm-text>
			{:else if !query.trim()}
				<mm-text variant="dense" tone="secondary" block>{m.lineage_search_hint()}</mm-text>
			{/if}
		</div>
	</div>
	<mm-button slot="footer" variant="secondary" data-close="cancel" disabled={adding}
		>{m.common_cancel()}</mm-button
	>
</mm-dialog>

<style>
	.results {
		min-height: 12rem;
		max-height: 22rem;
		overflow-y: auto;
	}
</style>
