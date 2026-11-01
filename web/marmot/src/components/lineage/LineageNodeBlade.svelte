<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime, formatNumber, formatRelativeTime } from '$lib/utils/format';
	import type { Asset } from '$lib/assets/types';
	import type { LineageEdge, LineageNode } from '$lib/lineage/types';
	import { assetHref, edgeKind, providerOf } from './graph';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		profile_picture?: string;
	}

	let {
		node,
		currentName,
		edges,
		onClose
	}: {
		node: LineageNode;
		/** Name of the asset whose lineage this is. */
		currentName: string;
		/** Stored edges directly between this asset and the current one. */
		edges: LineageEdge[];
		onClose: () => void;
	} = $props();

	let detail = $state<Asset | null>(null);
	let owners = $state<Owner[]>([]);

	// The lineage response carries most of the asset; the full record adds notes and sources, and owners come separately.
	$effect(() => {
		const id = node.asset.id;
		let cancelled = false;
		detail = null;
		owners = [];
		Promise.all([
			fetchApi(`/assets/${encodeURIComponent(id)}`).then((r) => (r.ok ? r.json() : null)),
			fetchApi(`/assets/owners/?asset_id=${encodeURIComponent(id)}`).then((r) =>
				r.ok ? r.json() : null
			)
		])
			.then(([a, o]) => {
				if (cancelled) return;
				detail = a;
				owners = o?.owners ?? [];
			})
			.catch((err) => console.error('Error loading asset details:', err));
		return () => {
			cancelled = true;
		};
	});

	let asset = $derived<Asset>(detail ?? node.asset);
	let provider = $derived(providerOf(asset));
	let href = $derived.by(() => {
		const path = assetHref(asset);
		return path ? resolve(path as `/discover/${string}/${string}/${string}`) : '';
	});
	let about = $derived(asset.user_description || asset.description || '');
	let pii = $derived((asset.tags ?? []).some((t) => t.toLowerCase() === 'pii'));
	let otherTags = $derived((asset.tags ?? []).filter((t) => t.toLowerCase() !== 'pii'));
	let typeLabel = $derived(
		asset.type ? asset.type.charAt(0).toUpperCase() + asset.type.slice(1) : ''
	);

	const KIND_LABELS: Record<string, () => string> = {
		FEEDS: m.lineage_rel_feeds,
		CONTAINS: m.lineage_rel_contains,
		PRODUCES: m.lineage_rel_produces,
		REFERENCES: m.lineage_rel_references,
		AGENT_LOOKUP: m.lineage_rel_looked_up
	};

	let relationship = $derived(
		[...new Set(edges.map((e) => edgeKind(e.type)))].map((k) => KIND_LABELS[k]()).join(', ')
	);
	let origins = $derived(new Set(edges.map((e) => e.origin ?? 'declared')));
	let recorded = $derived(
		origins.size === 0
			? ''
			: origins.size > 1
				? m.lineage_origin_both()
				: origins.has('observed')
					? m.lineage_origin_observed_word()
					: m.lineage_origin_declared_word()
	);
	let observations = $derived(edges.reduce((s, e) => s + (e.observation_count ?? 0), 0));
	let lastSeen = $derived(
		edges
			.map((e) => e.last_seen_at ?? '')
			.sort()
			.pop() ?? ''
	);
	let job = $derived(edges.find((e) => e.job_mrn)?.job_mrn ?? '');
	let declaredBy = $derived(
		[...new Set(edges.map((e) => e.declared_by).filter((d): d is string => !!d))].join(', ')
	);
	// Size reads "1,284 rows · 9 columns", from what the asset's plugin recorded.
	let size = $derived(
		[
			asset.row_count != null
				? m.asset_size_rows({ count: asset.row_count, formatted: formatNumber(asset.row_count) })
				: '',
			asset.column_count != null
				? m.asset_size_columns({
						count: asset.column_count,
						formatted: formatNumber(asset.column_count)
					})
				: ''
		]
			.filter(Boolean)
			.join(' · ')
	);
	const RUN_WORDS: Record<string, () => string> = {
		succeeded: m.asset_page_run_succeeded,
		failed: m.asset_page_run_failed,
		running: m.asset_page_run_running
	};
	let lastRun = $derived(node.last_run_status ? (RUN_WORDS[node.last_run_status]?.() ?? '') : '');
</script>

<mm-details-blade closable onmm-close={onClose}>
	<span slot="heading" class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
		{#if provider}
			<mm-provider-logo {provider} size="20" decorative></mm-provider-logo>
		{/if}
		{asset.name}
	</span>

	<mm-blade-section heading={m.lineage_blade_at_a_glance()}>
		<mm-description-list>
			<mm-description term={m.common_type()}>{typeLabel}</mm-description>
			{#if asset.providers?.length}
				<mm-description term={m.lineage_blade_provider()}
					>{asset.providers.join(', ')}</mm-description
				>
			{/if}
			{#if pii}
				<mm-description term={m.lineage_blade_personal_data()}>
					<mm-tag variant="pii"></mm-tag>
				</mm-description>
			{/if}
			{#if size}
				<mm-description term={m.lineage_blade_size()}>{size}</mm-description>
			{/if}
			{#if lastRun}
				<mm-description term={m.lineage_blade_last_run()}>
					{#if node.last_run_status === 'failed'}
						<mm-status-pill state="failed"></mm-status-pill>
					{:else}
						{lastRun}
					{/if}
				</mm-description>
			{/if}
			{#if asset.updated_at}
				<mm-description term={m.lineage_blade_updated()}
					>{formatRelativeTime(asset.updated_at)}</mm-description
				>
			{/if}
		</mm-description-list>
	</mm-blade-section>

	{#if about}
		<mm-blade-section heading={m.lineage_blade_about()}>
			<mm-text variant="dense" block>{about}</mm-text>
		</mm-blade-section>
	{/if}

	<mm-blade-section heading={m.lineage_blade_who_to_ask()}>
		{#if owners.length}
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				{#each owners as o (o.id)}
					<mm-owner-chip
						name={o.name}
						kind={o.type === 'team' ? 'team' : 'person'}
						src={o.profile_picture ?? ''}
					></mm-owner-chip>
				{/each}
			</div>
		{:else}
			<mm-text variant="dense" tone="secondary" block>{m.lineage_blade_no_owners()}</mm-text>
		{/if}
	</mm-blade-section>

	<mm-blade-section heading={m.lineage_blade_connects({ name: currentName })}>
		<mm-description-list>
			<mm-description term={m.lineage_blade_direction()}>
				{node.depth < 0 ? m.asset_lineage_upstream() : m.asset_lineage_downstream()}
			</mm-description>
			{#if Math.abs(node.depth) > 1}
				<mm-description term={m.lineage_blade_steps()}>
					<mm-text numeric>{formatNumber(Math.abs(node.depth))}</mm-text>
				</mm-description>
			{/if}
			{#if relationship}
				<mm-description term={m.lineage_blade_relationship()}>{relationship}</mm-description>
			{/if}
			{#if recorded}
				<mm-description term={m.lineage_blade_recorded()}>{recorded}</mm-description>
			{/if}
			{#if declaredBy}
				<mm-description term={m.lineage_blade_declared_by()}>{declaredBy}</mm-description>
			{/if}
			{#if observations}
				<mm-description term={m.lineage_blade_observed_in()}
					>{m.lineage_runs_count({
						count: observations,
						formatted: formatNumber(observations)
					})}</mm-description
				>
			{/if}
		</mm-description-list>
	</mm-blade-section>

	<mm-blade-section heading={m.lineage_blade_more_details()} collapsible>
		<mm-description-list>
			<mm-description term={m.lineage_blade_mrn()} mono>{asset.mrn}</mm-description>
			{#if otherTags.length}
				<mm-description term={m.common_tags()}>{otherTags.join(', ')}</mm-description>
			{/if}
			{#if asset.created_by}
				<mm-description term={m.lineage_blade_created_by()}>{asset.created_by}</mm-description>
			{/if}
			{#if job}
				<mm-description term={m.lineage_blade_job()} mono>{job}</mm-description>
			{/if}
			{#if lastSeen}
				<mm-description term={m.lineage_blade_edge_last_seen()}
					>{formatDateTime(lastSeen)}</mm-description
				>
			{/if}
		</mm-description-list>
	</mm-blade-section>

	{#if href}
		<div slot="footer" class="mm-cluster" style="--mm-gap: var(--space-2)">
			<mm-button variant="primary" icon="open-in-full" {href}
				>{m.lineage_blade_open_asset()}</mm-button
			>
		</div>
	{/if}
</mm-details-blade>
