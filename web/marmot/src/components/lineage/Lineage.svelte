<script lang="ts">
	import '@marmotdata/elements/lineage-canvas.js';
	import type {
		MmLineageCanvas,
		MmLineageActivateDetail,
		MmLineageAddDetail,
		MmLineageEdgeDeleteDetail,
		MmLineageFullscreenDetail,
		MmLineageOptionsDetail,
		MmLineageSelectDetail
	} from '@marmotdata/elements/classes/lineage-canvas.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import { onMount, untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { formatNumber } from '$lib/utils/format';
	import { BladeSlot } from '$lib/mm';
	import type { Asset } from '$lib/assets/types';
	import type { LineageResponse } from '$lib/lineage/types';
	import { buildGraph, edgeKind, isGroupId } from './graph';
	import LineageNodeBlade from './LineageNodeBlade.svelte';
	import AddLineageDialog from './AddLineageDialog.svelte';

	let { currentAsset }: { currentAsset: Asset } = $props();

	const canManageAssets = $derived(auth.hasPermission('assets', 'manage'));
	const isAgentAsset = $derived(currentAsset.type?.toLowerCase() === 'agent');
	const currentId = $derived(currentAsset.mrn || currentAsset.id);

	let canvas = $state<MmLineageCanvas | null>(null);
	let lineageData = $state<LineageResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let depth = $state(10);
	let fetchedDepth = 0;
	// Observed edges (agent lookups, OpenLineage runs) start on for agents and off elsewhere, so chatty agents don't crowd pipeline lineage.
	let showObserved = $state(false);
	let showStructure = $state(true);
	let selectedId = $state('');
	let full = $state(false);
	let addOpen = $state(false);
	let addDirection = $state<'upstream' | 'downstream'>('upstream');
	let addNodeId = $state('');
	const expandedGroups = new SvelteSet<string>();
	let refetchTimer: ReturnType<typeof setTimeout> | undefined;
	let fetchSeq = 0;

	async function fetchLineage() {
		const mine = ++fetchSeq;
		const asked = depth;
		loading = true;
		error = '';
		try {
			const res = await fetchApi(
				`/lineage/assets/${encodeURIComponent(currentAsset.id)}?depth=${asked}`
			);
			if (!res.ok) throw new Error(m.lineage_fetch_error());
			const data: LineageResponse = await res.json();
			if (mine !== fetchSeq) return;
			lineageData = data;
			fetchedDepth = asked;
		} catch (err) {
			if (mine !== fetchSeq) return;
			console.error('Error fetching lineage:', err);
			error = m.lineage_fetch_error();
			// With a graph already drawn the canvas keeps it, so the failed refresh is reported on its own.
			if (lineageData) toasts.error(error);
		} finally {
			if (mine === fetchSeq) loading = false;
		}
	}

	$effect(() => {
		void currentAsset.id;
		untrack(() => {
			lineageData = null;
			selectedId = '';
			fetchedDepth = 0;
			expandedGroups.clear();
			showObserved = isAgentAsset;
			fetchLineage();
		});
	});

	onMount(() => {
		full = page.url.searchParams.get('view') === 'full';
		return () => clearTimeout(refetchTimer);
	});

	let graph = $derived(
		buildGraph(lineageData, currentAsset, {
			groupNeighbours: isAgentAsset,
			expanded: expandedGroups,
			groupName: (count, type) => m.lineage_cluster_asset_count({ count, type })
		})
	);

	// The canvas takes its graph as a property only, so it is handed over directly rather than through an attribute.
	$effect(() => {
		if (canvas) canvas.graph = graph;
	});

	function nameOf(id: string): string {
		return lineageData?.nodes.find((n) => n.id === id)?.asset?.name || id.split('/').pop() || id;
	}

	let selectedNode = $derived.by(() => {
		if (!selectedId || selectedId === currentId || isGroupId(selectedId)) return null;
		const n = lineageData?.nodes.find((x) => x.id === selectedId);
		return n && !n.asset?.is_stub ? n : null;
	});

	let selectedEdges = $derived(
		selectedNode
			? (lineageData?.edges ?? []).filter(
					(e) =>
						(e.source === selectedNode.id && e.target === currentId) ||
						(e.target === selectedNode.id && e.source === currentId)
				)
			: []
	);

	let caption = $derived.by(() => {
		if (error && !lineageData) return m.lineage_caption_not_loaded();
		if (!lineageData) return '';
		const assets = Math.max(lineageData.nodes.length, 1);
		return [
			m.lineage_caption_assets({ count: assets, formatted: formatNumber(assets) }),
			m.lineage_caption_edges({
				count: lineageData.edges.length,
				formatted: formatNumber(lineageData.edges.length)
			}),
			m.lineage_caption_depth({ depth: formatNumber(depth) })
		].join(' · ');
	});

	function onSelect(e: CustomEvent<MmLineageSelectDetail>) {
		const id = e.detail.id ?? '';
		// Groups expand on activation; leaving them selected would offer to add lineage to something that is not an asset.
		if (id && isGroupId(id)) {
			if (canvas) canvas.selected = selectedId;
			return;
		}
		selectedId = id;
	}

	function onActivate(e: CustomEvent<MmLineageActivateDetail>) {
		const id = e.detail.id;
		if (isGroupId(id)) {
			if (expandedGroups.has(id)) expandedGroups.delete(id);
			else expandedGroups.add(id);
			return;
		}
		selectedId = id;
		if (canvas) canvas.selected = id;
	}

	function onOptions(e: CustomEvent<MmLineageOptionsDetail>) {
		depth = e.detail.depth;
		showObserved = e.detail.showObserved;
		showStructure = e.detail.showStructure;
		// The server cut the graph at the depth it was asked for, so a deeper view needs a new request.
		if (depth > fetchedDepth) {
			clearTimeout(refetchTimer);
			refetchTimer = setTimeout(fetchLineage, 300);
		}
	}

	function setFull(on: boolean) {
		full = on;
		const url = new URL(page.url);
		if (on) url.searchParams.set('view', 'full');
		else url.searchParams.delete('view');
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- the current page's own URL with only the view parameter changed
		replaceState(url, page.state);
	}

	function onFullscreen(e: CustomEvent<MmLineageFullscreenDetail>) {
		setFull(e.detail.fullscreen);
	}

	$effect(() => {
		if (!full) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key !== 'Escape' || e.defaultPrevented) return;
			// The canvas, menus and dialogs handle their own Escape first.
			const path = e.composedPath();
			if (
				path.some(
					(t) =>
						t instanceof HTMLElement &&
						/^MM-(LINEAGE-CANVAS|MENU|DIALOG|CONFIRM-DIALOG)$/.test(t.tagName)
				)
			)
				return;
			setFull(false);
		};
		document.addEventListener('keydown', onKey);
		return () => document.removeEventListener('keydown', onKey);
	});

	function onAddLineage(e: CustomEvent<MmLineageAddDetail>) {
		addNodeId = e.detail.id;
		addDirection = e.detail.direction;
		addOpen = true;
	}

	async function addLineage(mrn: string) {
		// Upstream means the picked asset feeds the selected one; downstream the other way round.
		const [source, target] = addDirection === 'upstream' ? [mrn, addNodeId] : [addNodeId, mrn];
		const res = await fetchApi('/lineage/direct', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ source, target })
		});
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			throw new Error(body.error || m.lineage_create_error());
		}
		toasts.success(m.lineage_added());
		await fetchLineage();
	}

	async function onEdgeDelete(e: CustomEvent<MmLineageEdgeDeleteDetail>) {
		const { source, target, kind } = e.detail;
		const candidates = (lineageData?.edges ?? []).filter(
			(x) => x.source === source && x.target === target
		);
		const edge = candidates.find((x) => edgeKind(x.type) === kind) ?? candidates[0];
		if (!edge) return;
		const ok = await MmConfirmDialog.confirm({
			heading: m.lineage_delete_heading(),
			body: m.lineage_delete_body({ source: nameOf(source), target: nameOf(target) }),
			confirmLabel: m.lineage_delete_edge()
		});
		if (!ok) return;
		try {
			const res = await fetchApi(`/lineage/direct/${encodeURIComponent(edge.id)}`, {
				method: 'DELETE'
			});
			if (!res.ok) throw new Error();
			toasts.success(m.lineage_deleted());
			await fetchLineage();
		} catch (err) {
			console.error('Error deleting lineage:', err);
			toasts.error(m.lineage_delete_error());
		}
	}

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(window.location.href);
			toasts.success(m.lineage_link_copied());
		} catch {
			toasts.error(m.lineage_link_copy_error());
		}
	}
</script>

<div class="lineage" class:lineage--full={full}>
	{#if full}
		<div class="lineage__head">
			<mm-page-header compact heading={m.lineage_of({ name: currentAsset.name })} {caption}>
				<mm-button
					slot="actions"
					variant="secondary"
					icon="arrow-back"
					onclick={() => setFull(false)}>{m.lineage_back_to({ name: currentAsset.name })}</mm-button
				>
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label={m.lineage_more_actions()}
				>
					<mm-menu onmm-select={copyLink}>
						<mm-menu-item icon="link">{m.lineage_copy_link()}</mm-menu-item>
					</mm-menu>
				</mm-menu-button>
			</mm-page-header>
		</div>
	{/if}
	<div class="lineage__body">
		<mm-lineage-canvas
			bind:this={canvas}
			class="lineage__canvas"
			label={m.lineage_of({ name: currentAsset.name })}
			{depth}
			max-depth={20}
			show-observed={showObserved}
			hide-structure={!showStructure}
			fullscreen={full}
			hide-minimap={!full}
			editable={canManageAssets}
			loading={loading && !lineageData}
			error={lineageData ? '' : error}
			selected={selectedId}
			onmm-node-select={onSelect}
			onmm-node-activate={onActivate}
			onmm-options-change={onOptions}
			onmm-fullscreen-change={onFullscreen}
			onmm-add-lineage={onAddLineage}
			onmm-edge-delete={onEdgeDelete}
			onmm-retry={fetchLineage}
		>
			{#if expandedGroups.size}
				<mm-button
					slot="actions"
					variant="ghost"
					size="sm"
					icon="expand-less"
					onclick={() => expandedGroups.clear()}>{m.lineage_collapse_groups()}</mm-button
				>
			{/if}
		</mm-lineage-canvas>
		{#if full && selectedNode}
			<LineageNodeBlade
				node={selectedNode}
				currentName={currentAsset.name}
				edges={selectedEdges}
				onClose={() => (selectedId = '')}
			/>
		{/if}
	</div>
</div>

{#if !full && selectedNode}
	<BladeSlot label={m.lineage_blade_label()}>
		<LineageNodeBlade
			node={selectedNode}
			currentName={currentAsset.name}
			edges={selectedEdges}
			onClose={() => (selectedId = '')}
		/>
	</BladeSlot>
{/if}

<AddLineageDialog
	bind:open={addOpen}
	direction={addDirection}
	assetName={nameOf(addNodeId)}
	onAdd={addLineage}
/>

<style>
	.lineage__body {
		display: flex;
		min-width: 0;
	}
	.lineage__canvas {
		flex: 1;
		min-width: 0;
		--mm-lineage-height: 540px;
	}
	.lineage--full {
		position: fixed;
		inset: 0;
		/* Above the shell's sidebar and top bar, below toasts so "Link copied" still shows. */
		z-index: var(--mm-z-popover);
		display: flex;
		flex-direction: column;
		background: var(--mm-paper);
	}
	.lineage__head {
		padding: var(--space-4) var(--space-8);
	}
	.lineage--full .lineage__body {
		flex: 1;
		min-height: 0;
	}
	/* The canvas's own full screen is fixed to the viewport; here it sits under the page header instead, edge to edge. */
	.lineage--full .lineage__canvas {
		position: relative;
		inset: auto;
		z-index: auto;
		height: auto;
		border-inline: 0;
		border-block-end: 0;
		border-radius: 0;
	}
	@media (max-width: 640px) {
		.lineage__head {
			padding: var(--space-4);
		}
	}
</style>
