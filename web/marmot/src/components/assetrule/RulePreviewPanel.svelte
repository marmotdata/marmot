<script lang="ts">
	import { resolve } from '$app/paths';
	import { fetchAssetBatch } from '$lib/dataproducts/assets';
	import { previewAssetRule } from '$lib/assetrules/api';
	import { assetCaption, assetHref } from '$lib/assetrules/assets';
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';

	let {
		query,
		onresult
	}: {
		query: string;
		/** Tells the editor whether the query is valid, so it can explain the problem beside the query and hold Save. */
		onresult?: (r: { count: number | null; error: string | null }) => void;
	} = $props();

	const SAMPLE_SIZE = 10;
	const DEBOUNCE_MS = 400;

	let loading = $state(false);
	let error = $state<string | null>(null);
	let count = $state<number | null>(null);
	let sample = $state<Asset[]>([]);

	let trimmed = $derived(query.trim());

	// A sequence number lets a slow response for an older query be discarded once the user has typed further
	let requestSeq = 0;

	async function runPreview(expression: string, seq: number) {
		try {
			const result = await previewAssetRule({
				rule_type: 'query',
				query_expression: expression,
				limit: SAMPLE_SIZE
			});
			const ids = (result.asset_ids || []).slice(0, SAMPLE_SIZE);
			const assets = await fetchAssetBatch(ids);
			if (seq !== requestSeq) return;
			count = result.asset_count;
			sample = assets;
			error = result.errors?.length ? result.errors.join(' ') : null;
		} catch (e) {
			if (seq !== requestSeq) return;
			count = null;
			sample = [];
			error = e instanceof Error ? e.message : m.assetrules_preview_error();
		} finally {
			if (seq === requestSeq) {
				loading = false;
				onresult?.({ count, error });
			}
		}
	}

	$effect(() => {
		const expression = trimmed;
		const seq = ++requestSeq;
		if (!expression) {
			loading = false;
			error = null;
			count = null;
			sample = [];
			onresult?.({ count: null, error: null });
			return;
		}
		loading = true;
		const timer = setTimeout(() => runPreview(expression, seq), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	});
</script>

<div aria-live="polite">
	{#if error}
		<mm-callout tone="warning" heading={m.assetrules_preview_paused_heading()}>
			{m.assetrules_preview_paused_body()}
		</mm-callout>
	{:else}
		<mm-panel
			heading={m.assetrules_preview_panel_heading()}
			icon="visibility"
			count={count ?? undefined}
			loading={loading && count === null}
			empty={!trimmed || count === 0}
		>
			{#if !trimmed}
				<mm-empty-state
					slot="empty"
					compact
					heading={m.assetrules_preview_empty_heading()}
					description={m.assetrules_preview_empty_hint()}
					seed={9}
				></mm-empty-state>
			{:else if count === 0}
				<mm-empty-state
					slot="empty"
					compact
					heading={m.assetrules_preview_no_matches_heading()}
					description={m.assetrules_preview_no_matches_hint()}
					seed={9}
				></mm-empty-state>
			{:else}
				<mm-list dense class:stale={loading}>
					{#each sample as asset (asset.id)}
						<mm-list-item href={assetHref(asset)} secondary={assetCaption(asset)}>
							{#if asset.providers?.[0]}
								<mm-provider-logo slot="start" provider={asset.providers[0]} size={16} decorative
								></mm-provider-logo>
							{/if}
							{asset.name}
						</mm-list-item>
					{/each}
				</mm-list>
				<div slot="footer">
					<mm-link href={`${resolve('/discover')}?q=${encodeURIComponent(trimmed)}`}
						>{m.assetrules_open_in_discover()}</mm-link
					>
				</div>
			{/if}
		</mm-panel>
	{/if}
</div>

<style>
	.stale {
		opacity: 0.6;
	}
</style>
