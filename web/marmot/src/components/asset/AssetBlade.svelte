<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { page } from '$app/state';
	import type { Asset, AssetTerm } from '$lib/assets/types';
	import type { LineageNode, LineageResponse } from '$lib/lineage/types';
	import RunHistory from '$components/runs/RunHistory.svelte';
	import AssetGlossaryTerms from './AssetGlossaryTerms.svelte';
	import SubscribeButton from './SubscribeButton.svelte';
	import { BladeSlot } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils';
	import { formatRelativeTime } from '$lib/utils/format';
	import { compactNumber, providerLabel, typeLabel } from '$components/query/catalogSearch';
	import '@marmotdata/elements/popularity.js';

	interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	// assetUrl lets a caller hand us an already-built discover URL; it is expected to be a `/discover/<type>/<service>/<name>` path.
	type DiscoverAssetRoute = `/discover/${string}/${string}/${string}`;

	interface Props {
		asset?: Asset | null;
		lineage?: LineageResponse | null;
		onClose: () => void;
		/** Sits in the asset page's grid instead of opening over the shell as a preview. */
		staticPlacement?: boolean;
		collapsed?: boolean;
		onToggleCollapse?: () => void;
		assetUrl?: string;
	}

	let {
		asset = $bindable(null),
		lineage = null,
		onClose,
		staticPlacement = false,
		collapsed = false,
		onToggleCollapse,
		assetUrl
	}: Props = $props();

	const canManageAssets = auth.hasPermission('assets', 'manage');
	const canQuery = auth.hasPermission('assets', 'query');

	let ownLineage = $state<LineageResponse | null>(null);
	let lineageError = $state(false);
	let owners = $state<Owner[]>([]);
	let terms = $state<AssetTerm[]>([]);
	let deleteDialog = $state<HTMLElement & { show: () => void; close: (v?: string) => void }>();
	let deleting = $state(false);
	let deleteError = $state('');

	let isRunHistoryTab = $derived(page.url.searchParams.get('tab') === 'run-history');

	/** The discover route for any asset: its type, then the MRN's service and the rest of the name. */
	function routeOf(a: Pick<Asset, 'mrn' | 'type'>): DiscoverAssetRoute | '' {
		if (!a?.mrn || !a?.type) return '';
		const parts = a.mrn.replace('mrn://', '').split('/');
		if (parts.length < 3) return '';
		return `/discover/${a.type.toLowerCase()}/${parts[1]}/${encodeURIComponent(parts.slice(2).join('/'))}`;
	}

	let fullViewUrl = $derived(
		asset ? ((assetUrl as DiscoverAssetRoute | undefined) ?? routeOf(asset)) : ''
	);
	let provider = $derived(asset?.providers?.[0] ?? '');
	let shownLineage = $derived(lineage ?? ownLineage);
	let neighbours = $derived(
		(shownLineage?.nodes ?? []).filter((n) => n.depth !== 0 && !n.asset?.is_stub)
	);
	let upstream = $derived(neighbours.filter((n) => n.depth < 0));
	let downstream = $derived(neighbours.filter((n) => n.depth > 0));
	let queryable = $derived(
		canQuery && !!asset && ['table', 'view'].includes(asset.type.toLowerCase())
	);
	let queryHref = $derived(
		asset
			? `${resolve('/query')}?${new URLSearchParams({ q: `SELECT *\nFROM ${asset.name}\nLIMIT 10` })}`
			: ''
	);
	let about = $derived(asset?.user_description || asset?.description || '');
	let tags = $derived((asset?.tags ?? []).filter((t) => t !== 'pii'));
	let popularity = $derived(
		asset?.popularity && asset.popularity >= 1
			? (Math.min(Math.round(asset.popularity), 4) as 1 | 2 | 3 | 4)
			: null
	);
	let size = $derived(
		[
			asset?.row_count
				? m.discover_result_rows({
						count: asset.row_count,
						rows: compactNumber(asset.row_count)
					})
				: '',
			asset?.column_count ? m.discover_result_columns({ count: asset.column_count }) : ''
		]
			.filter(Boolean)
			.join(' · ')
	);

	// Each asset gets its own lineage, owners and terms; a newer asset wins if two loads overlap.
	$effect(() => {
		const id = asset?.id;
		ownLineage = null;
		lineageError = false;
		owners = [];
		terms = [];
		if (!id) return;
		let live = true;
		if (!lineage) {
			fetchApi(`/lineage/assets/${id}?depth=1`)
				.then((r) => (r.ok ? r.json() : Promise.reject()))
				.then((data: LineageResponse) => live && (ownLineage = data))
				.catch(() => live && (lineageError = true));
		}
		fetchApi(`/assets/owners/?asset_id=${id}`)
			.then((r) => (r.ok ? r.json() : { owners: [] }))
			.then((data: { owners?: Owner[] }) => live && (owners = data.owners ?? []))
			.catch(() => {});
		if (!staticPlacement) {
			fetchApi(`/assets/terms/${id}`)
				.then((r) => (r.ok ? r.json() : []))
				.then((data: AssetTerm[] | null) => live && (terms = data ?? []))
				.catch(() => {});
		}
		return () => {
			live = false;
		};
	});

	async function handleDelete(e: Event) {
		e.preventDefault();
		if (!asset?.id) return;
		deleting = true;
		deleteError = '';
		try {
			const response = await fetchApi(`/assets/${asset.id}`, { method: 'DELETE' });
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || m.asset_delete_error());
			}
			deleteDialog?.close('confirm');
			if (staticPlacement) {
				goto(resolve('/discover'));
			} else {
				onClose();
				// The list behind the preview still shows the deleted asset, so reload it.
				window.location.reload();
			}
		} catch (err) {
			deleteError = err instanceof Error ? err.message : m.asset_delete_error();
		} finally {
			deleting = false;
		}
	}

	function onMenu(e: CustomEvent<{ value: string }>) {
		if (e.detail.value === 'delete') deleteDialog?.show();
	}

	function ownerKind(o: Owner): 'person' | 'team' {
		return o.type === 'team' ? 'team' : 'person';
	}

	function nodeCaption(n: LineageNode): string {
		const where = n.depth < 0 ? m.asset_lineage_upstream() : m.asset_lineage_downstream();
		const kind = [
			n.asset.providers?.[0] ? providerLabel(n.asset.providers[0]) : '',
			typeLabel(n.asset.type)
		]
			.filter(Boolean)
			.join(' ');
		return `${where} · ${kind}`;
	}
</script>

{#snippet details(a: Asset)}
	<mm-blade-section heading={m.discover_blade_more_details()} collapsible>
		<mm-description-list>
			<mm-description term={m.discover_blade_mrn()} mono>{a.mrn}</mm-description>
			{#if a.parent_mrn}
				<mm-description term={m.discover_blade_parent()} mono>{a.parent_mrn}</mm-description>
			{/if}
			<mm-description term={m.discover_blade_created_by()}
				>{a.created_by || m.common_unknown()}</mm-description
			>
			<mm-description term={m.discover_blade_created()}
				>{a.created_at ? formatDateTime(a.created_at) : m.common_unknown()}</mm-description
			>
			<mm-description term={m.discover_blade_updated()}
				>{a.updated_at ? formatDateTime(a.updated_at) : m.common_unknown()}</mm-description
			>
		</mm-description-list>
	</mm-blade-section>
{/snippet}

{#snippet flows()}
	{#if neighbours.length}
		<mm-blade-section heading={m.discover_blade_where_it_flows()} flush>
			{#if fullViewUrl}
				<mm-link slot="actions" href={`${resolve(fullViewUrl as DiscoverAssetRoute)}?tab=lineage`}
					>{m.discover_blade_open_lineage()}</mm-link
				>
			{/if}
			<mm-list dense aria-label={m.discover_blade_nearest_lineage()}>
				{#each [...upstream, ...downstream].slice(0, 8) as n (n.id)}
					{@const route = routeOf(n.asset)}
					<mm-list-item
						href={route ? resolve(route as DiscoverAssetRoute) : ''}
						secondary={nodeCaption(n)}
					>
						{#if n.asset.providers?.[0]}
							<mm-provider-logo
								slot="start"
								provider={n.asset.providers[0].toLowerCase()}
								decorative
							></mm-provider-logo>
						{:else}
							<mm-icon slot="start" name="database" aria-hidden="true"></mm-icon>
						{/if}
						{n.asset.name}
					</mm-list-item>
				{/each}
			</mm-list>
		</mm-blade-section>
	{:else if lineageError}
		<mm-blade-section heading={m.discover_blade_where_it_flows()}>
			<mm-text variant="dense" tone="secondary">{m.asset_lineage_load_error()}</mm-text>
		</mm-blade-section>
	{/if}
{/snippet}

{#if asset && staticPlacement}
	<mm-details-blade
		heading={m.discover_blade_static_heading()}
		collapsible
		{collapsed}
		onmm-collapse={() => onToggleCollapse?.()}
	>
		{#if canManageAssets}
			<mm-menu-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="more-horiz"
				icon-only
				label={m.discover_blade_more({ name: asset?.name ?? '' })}
			>
				<mm-menu onmm-select={onMenu}>
					<mm-menu-item icon="delete" danger value="delete"
						>{m.discover_blade_delete()}</mm-menu-item
					>
				</mm-menu>
			</mm-menu-button>
		{/if}
		{#if asset.description}
			<mm-blade-section heading={m.discover_blade_technical_description()}>
				<mm-tag slot="actions" variant="plugin"></mm-tag>
				<mm-text variant="dense" block style="white-space: pre-wrap">{asset.description}</mm-text>
			</mm-blade-section>
		{/if}
		<AssetGlossaryTerms {asset} editable={true} />
		{#if !isRunHistoryTab && asset.has_run_history}
			<mm-blade-section heading={m.discover_blade_runs()}>
				<RunHistory assetId={asset.id} minimal={true} {asset} />
			</mm-blade-section>
		{/if}
		{@render flows()}
		{@render details(asset)}
	</mm-details-blade>
{:else if asset}
	<BladeSlot label={m.discover_blade_preview_label()}>
		<mm-details-blade
			mode="overlay"
			open
			resizable
			storage-key="asset-preview"
			onmm-close={onClose}
		>
			<span slot="heading" class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
				{#if provider}
					<mm-provider-logo provider={provider.toLowerCase()} size={20} decorative
					></mm-provider-logo>
				{/if}
				{asset.name}
			</span>
			<span slot="actions" class="mm-cluster" style="--mm-gap: var(--space-1)">
				<SubscribeButton assetId={asset.id} variant="icon-only" />
			</span>
			{#if canManageAssets}
				<mm-menu-button
					slot="actions"
					variant="ghost"
					size="sm"
					icon="more-horiz"
					icon-only
					label={m.discover_blade_more({ name: asset?.name ?? '' })}
				>
					<mm-menu onmm-select={onMenu}>
						<mm-menu-item icon="delete" danger value="delete"
							>{m.discover_blade_delete()}</mm-menu-item
						>
					</mm-menu>
				</mm-menu-button>
			{/if}

			<mm-blade-section heading={m.discover_blade_at_a_glance()}>
				<mm-description-list>
					<mm-description term={m.common_type()}
						>{[typeLabel(asset.type), ...(asset.providers ?? []).map(providerLabel)].join(
							' · '
						)}</mm-description
					>
					<mm-description term={m.discover_blade_freshness()}>
						{#if asset.stale}
							<mm-status-pill state="stale"></mm-status-pill>
						{:else if asset.last_sync_at || asset.updated_at}
							{m.discover_result_updated({
								when: formatRelativeTime(asset.last_sync_at || asset.updated_at)
							})}
						{:else}
							{m.common_unknown()}
						{/if}
					</mm-description>
					<mm-description term={m.discover_blade_personal_data()}>
						{#if asset.tags?.includes('pii')}
							<mm-tag variant="pii"></mm-tag>
						{:else}
							{m.discover_blade_personal_data_none()}
						{/if}
					</mm-description>
					{#if asset.popularity !== undefined}
						<mm-description term={m.discover_blade_popularity()}>
							{#if popularity}
								<mm-popularity level={popularity}></mm-popularity>
							{:else}
								{m.discover_blade_popularity_none()}
							{/if}
						</mm-description>
					{/if}
					{#if size}
						<mm-description term={m.discover_blade_size()}>{size}</mm-description>
					{/if}
				</mm-description-list>
			</mm-blade-section>

			{#if about}
				<mm-blade-section heading={m.discover_blade_about()}>
					<mm-text variant="dense" block style="white-space: pre-wrap">{about}</mm-text>
				</mm-blade-section>
			{/if}

			{#if owners.length}
				<mm-blade-section heading={m.discover_blade_who_to_ask()}>
					<div class="mm-stack" style="--mm-gap: var(--space-3)">
						{#each owners as o (o.type + o.id)}
							<mm-owner-chip
								name={o.name || o.username || ''}
								kind={ownerKind(o)}
								src={o.profile_picture ?? ''}
								email={o.email ?? ''}
							></mm-owner-chip>
						{/each}
					</div>
				</mm-blade-section>
			{/if}

			{#if terms.length || tags.length}
				<mm-blade-section heading={m.discover_blade_what_it_means()}>
					<div class="mm-cluster" style="--mm-gap: var(--space-2)">
						{#each terms as t (t.term_id)}
							<mm-tag href={resolve('/glossary/[[id]]', { id: t.term_id })}>{t.term_name}</mm-tag>
						{/each}
						{#each tags as t (t)}
							<mm-tag>{t}</mm-tag>
						{/each}
					</div>
				</mm-blade-section>
			{/if}

			{@render flows()}
			{@render details(asset)}

			<div slot="footer" class="mm-cluster" style="--mm-gap: var(--space-2)">
				{#if fullViewUrl}
					<mm-button
						variant="primary"
						icon="open-in-full"
						href={resolve(fullViewUrl as DiscoverAssetRoute)}
						>{m.discover_blade_open_asset()}</mm-button
					>
				{/if}
				{#if queryable}
					<mm-button icon="terminal" href={queryHref}>{m.discover_blade_query()}</mm-button>
				{/if}
			</div>
		</mm-details-blade>
	</BladeSlot>
{/if}

{#if asset && canManageAssets}
	<mm-confirm-dialog
		bind:this={deleteDialog}
		tone="danger"
		heading={m.discover_blade_delete()}
		confirm-label={m.discover_blade_delete()}
		confirm-icon="delete"
		loading={deleting}
		onmm-confirm={handleDelete}
	>
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			<mm-text variant="body" block>{m.discover_blade_delete_body({ name: asset.name })}</mm-text>
			{#if deleteError}
				<mm-callout tone="danger" compact>{deleteError}</mm-callout>
			{/if}
		</div>
	</mm-confirm-dialog>
{/if}
