<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import type { LineageNode, LineageResponse } from '$lib/lineage/types';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime, formatRelativeTime } from '$lib/utils';
	import AssetDescriptions from './AssetDescriptions.svelte';
	import AssetGlossaryTerms from './AssetGlossaryTerms.svelte';
	import { assetHref, providerKey } from './assetFormat';

	// The asset page's blade: what decides whether the asset can be used, then what it means and where it flows; owners, tags and freshness live in the page header so they are not repeated here.
	let {
		asset,
		collapsed = false,
		oncollapse
	}: { asset: Asset; collapsed?: boolean; oncollapse?: (collapsed: boolean) => void } = $props();

	interface LatestRun {
		status: string;
		start_time?: string;
		event_time: string;
	}

	const NEAREST_LIMIT = 5;

	let lineage = $state<LineageResponse | null>(null);
	let lineageError = $state(false);
	let latestRun = $state<LatestRun | null>(null);

	let isAgent = $derived((asset.type || '').toLowerCase() === 'agent');
	let upstream = $derived(lineage?.nodes.filter((n) => n.depth < 0) ?? []);
	let downstream = $derived(lineage?.nodes.filter((n) => n.depth > 0) ?? []);
	let nearest = $derived([
		...upstream.slice(0, NEAREST_LIMIT).map((n) => ({ n, dir: m.asset_lineage_upstream() })),
		...downstream.slice(0, NEAREST_LIMIT).map((n) => ({ n, dir: m.asset_lineage_downstream() }))
	]);

	$effect(() => {
		const id = asset.id;
		lineage = null;
		lineageError = false;
		fetchApi(`/lineage/assets/${id}?depth=1`)
			.then(async (r) => {
				if (!r.ok) throw new Error(m.asset_lineage_load_error());
				lineage = await r.json();
			})
			.catch(() => (lineageError = true));
	});

	$effect(() => {
		const id = asset.id;
		latestRun = null;
		if (!asset.has_run_history) return;
		fetchApi(`/assets/run-history/${id}?limit=1&offset=0`)
			.then(async (r) => {
				if (!r.ok) return;
				const data = await r.json();
				latestRun = data.run_history?.[0] ?? null;
			})
			.catch(() => {});
	});

	function runWord(status: string): string {
		switch (status.toUpperCase()) {
			case 'COMPLETE':
				return m.asset_page_run_succeeded();
			case 'FAIL':
				return m.asset_page_run_failed();
			case 'RUNNING':
				return m.asset_page_run_running();
			default:
				return status;
		}
	}

	function nodeSecondary(n: LineageNode, dir: string): string {
		if (n.asset?.is_stub) return `${dir} · ${m.asset_page_not_catalogued()}`;
		const type = n.asset?.type;
		const what =
			n.last_run_status === 'failed' && type
				? `${type}, ${m.asset_page_last_run_failed_hint()}`
				: type;
		return [dir, what].filter(Boolean).join(' · ');
	}

	function href(n: LineageNode): string | undefined {
		if (!n.asset || n.asset.is_stub) return undefined;
		return assetHref(n.asset);
	}
</script>

<mm-details-blade
	heading={isAgent ? m.asset_page_blade_agent_heading() : m.asset_page_blade_heading()}
	collapsible
	{collapsed}
	resizable
	storage-key="marmot-asset-blade-width"
	onmm-collapse={(e: CustomEvent<{ collapsed: boolean }>) => oncollapse?.(e.detail.collapsed)}
>
	{#if latestRun || asset.last_sync_at}
		<mm-blade-section heading={m.asset_page_at_a_glance()}>
			<mm-description-list>
				{#if latestRun}
					<mm-description term={m.asset_page_last_run()}
						>{runWord(latestRun.status)} · {formatRelativeTime(
							latestRun.start_time || latestRun.event_time
						)}</mm-description
					>
				{/if}
				{#if asset.last_sync_at}
					<mm-description term={m.asset_page_last_synced()}
						>{formatRelativeTime(asset.last_sync_at)}</mm-description
					>
				{/if}
			</mm-description-list>
		</mm-blade-section>
	{/if}

	<AssetDescriptions {asset} showNotes={false} />

	<AssetGlossaryTerms {asset} />

	<mm-blade-section
		heading={isAgent ? m.asset_page_what_it_reads() : m.asset_page_where_it_flows()}
		flush={nearest.length > 0}
	>
		{#if nearest.length > 0}
			<mm-link slot="actions" href="?tab=lineage">{m.asset_page_open_lineage()}</mm-link>
			<mm-list dense aria-label={m.asset_page_nearest_lineage()}>
				{#each nearest as { n, dir } (n.id)}
					<mm-list-item href={href(n)} secondary={nodeSecondary(n, dir)}>
						<mm-provider-logo slot="start" provider={providerKey(n.asset)} size="20" decorative
						></mm-provider-logo>
						{n.asset?.name ?? n.id}
					</mm-list-item>
				{/each}
			</mm-list>
		{:else if lineageError}
			<mm-text variant="dense" tone="secondary" block>{m.asset_lineage_load_error()}</mm-text>
		{:else if lineage}
			<mm-text variant="dense" tone="secondary" block>{m.asset_page_no_lineage()}</mm-text>
		{:else}
			<mm-skeleton count={3}></mm-skeleton>
		{/if}
	</mm-blade-section>

	<mm-blade-section heading={m.asset_page_more_details()} collapsible>
		<mm-description-list>
			<mm-description term={m.asset_page_created_by()}
				>{asset.created_by || m.common_unknown()}</mm-description
			>
			<mm-description term={m.asset_page_created()}
				>{asset.created_at ? formatDateTime(asset.created_at) : m.common_unknown()}</mm-description
			>
			<mm-description term={m.asset_page_updated()}
				>{asset.updated_at ? formatDateTime(asset.updated_at) : m.common_unknown()}</mm-description
			>
			{#if asset.parent_mrn}
				<mm-description term={m.asset_page_parent()} mono>{asset.parent_mrn}</mm-description>
			{/if}
		</mm-description-list>
	</mm-blade-section>
</mm-details-blade>
