<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import type { Asset } from '$lib/assets/types';
	import { page } from '$app/stores';
	import { SvelteFlow, Background, type Node, type Edge, type NodeTypes } from '@xyflow/svelte';
	import '@xyflow/svelte/dist/style.css';
	import dagre from '@dagrejs/dagre';
	import AssetBlade from './AssetBlade.svelte';
	import CustomNode from './CustomNode.svelte';
	import Button from './Button.svelte';

	export let currentAsset: Asset;
	let selectedAsset: Asset | null = null;
	let loading = true;
	let error: string | null = null;
	let mounted = false;
	let lineageData: LineageResponse | null = null;
	let depth = 10;
	let initialLoad = true;

	const nodes = writable<Node[]>([]);
	const edges = writable<Edge[]>([]);

	function getNodeIconType(node: any): string {
		if (
			node.asset.providers &&
			Array.isArray(node.asset.providers) &&
			node.asset.providers.length === 1
		) {
			return node.asset.providers[0];
		}
		return node.asset.type;
	}

	const nodeTypes: NodeTypes = {
		custom: CustomNode
	};

	$: {
		$page;
		selectedAsset = null;
	}

  async function handleNodeClick(nodeId: string) {
    if (nodeId && nodeId !== currentAsset.id) {
			try {
				const [assetResponse, lineageResponse] = await Promise.all([
					fetchApi(`/assets/${nodeId}`),
					fetchApi(`/lineage/assets/${nodeId}`)
				]);

				if (!assetResponse.ok) throw new Error('Failed to fetch asset');
				if (!lineageResponse.ok) throw new Error('Failed to fetch lineage');

				selectedAsset = await assetResponse.json();
				lineageData = await lineageResponse.json();
			} catch (error) {
				console.error('Error fetching asset/lineage:', error);
			}
		}
	}

	function findCycles(edges: Edge[]): Map<string, Set<string>> {
		const cycleMap = new Map<string, Set<string>>();

		edges.forEach((edge) => {
			const reverseEdge = edges.find((e) => e.source === edge.target && e.target === edge.source);
			if (reverseEdge) {
				if (!cycleMap.has(edge.source)) {
					cycleMap.set(edge.source, new Set());
				}
				if (!cycleMap.has(edge.target)) {
					cycleMap.set(edge.target, new Set());
				}
				cycleMap.get(edge.source)?.add(edge.target);
				cycleMap.get(edge.target)?.add(edge.source);
			}
		});

		return cycleMap;
	}

	function getLayoutedElements(nodes: Node[], edges: Edge[]) {
		const g = new dagre.graphlib.Graph();
		g.setGraph({
			rankdir: 'LR',
			nodesep: 120,
			ranksep: 150,
			marginx: 50,
			marginy: 50
		});

		g.setDefaultEdgeLabel(() => ({}));
		const cycles = findCycles(edges);
		const primaryNodes = findPrimaryNodes(nodes, edges, cycles);
		const verticalGap = 150;

		// Add nodes to graph first
		nodes.forEach((node) => {
			const dimensions = {
				width: Math.max(180, node.data.name.length * 10 + 60),
				height: 80
			};
			g.setNode(node.id, dimensions);
		});

		// Add non-cyclic edges
		edges.forEach((edge) => {
			const sourceCycles = cycles.get(edge.source);
			const targetCycles = cycles.get(edge.target);

			if (!sourceCycles?.has(edge.target) && !targetCycles?.has(edge.source)) {
				g.setEdge(edge.source, edge.target);
			}
		});

		dagre.layout(g);

		// Now position nodes
		return nodes.map((node) => {
			const nodeWithPosition = g.node(node.id);
			const baseX = nodeWithPosition.x - nodeWithPosition.width / 2;
			const baseY = nodeWithPosition.y - nodeWithPosition.height / 2;

			if (cycles.has(node.id) && !primaryNodes.has(node.id)) {
				const primaryNodeId = Array.from(cycles.get(node.id) || []).find((id) =>
					primaryNodes.has(id)
				);

				if (primaryNodeId) {
					const primaryPos = g.node(primaryNodeId);
					return {
						...node,
						position: {
							x: primaryPos.x - nodeWithPosition.width / 2,
							y: primaryPos.y + verticalGap
						}
					};
				}
			}

			return {
				...node,
				position: { x: baseX, y: baseY }
			};
		});
	}

	function findPrimaryNodes(
		nodes: Node[],
		edges: Edge[],
		cycles: Map<string, Set<string>>
	): Set<string> {
		const primaryNodes = new Set<string>();

		nodes.forEach((node) => {
			const incomingEdges = edges.filter((e) => e.target === node.id);
			const hasNonCyclicInput = incomingEdges.some(
				(edge) => !cycles.has(edge.source) || !cycles.get(edge.source)?.has(node.id)
			);

			if (hasNonCyclicInput) {
				primaryNodes.add(node.id);
			}
		});

		return primaryNodes;
	}

	function generateElements(data: LineageResponse) {
		const connections = new Map<string, { hasUpstream: boolean; hasDownstream: boolean }>();

		// If there's no edges, create a single node for the current asset
		if (!data.edges || data.edges.length === 0) {
			return {
				nodes: [
					{
						id: currentAsset.mrn || currentAsset.id,
						type: 'custom',
						data: {
							id: currentAsset.id,
							name: currentAsset.name || '',
							type: currentAsset.providers?.[0] || currentAsset.type,
							provider: currentAsset.provider,
							isCurrent: true,
							depth: 0,
							hasUpstream: false,
							hasDownstream: false
						},
						position: { x: 0, y: 0 }
					}
				],
				edges: []
			};
		}

		data.edges.forEach((edge) => {
			connections.set(edge.source, {
				hasUpstream: connections.get(edge.source)?.hasUpstream || false,
				hasDownstream: true
			});
			connections.set(edge.target, {
				hasUpstream: true,
				hasDownstream: connections.get(edge.target)?.hasDownstream || false
			});
		});

		const nodes = data.nodes.map((node) => {
			const nodeConnections = connections.get(node.id) || {
				hasUpstream: false,
				hasDownstream: false
			};
      return {
          id: node.id,
          type: 'custom',
          data: {
              id: node.asset.id,
              name: node.asset.name || '',
              type: getNodeIconType(node),
              provider: node.asset.provider,
              isCurrent: node.id === currentAsset.mrn,
              depth: node.depth,
              hasUpstream: nodeConnections.hasUpstream,
              hasDownstream: nodeConnections.hasDownstream,
              nodeClickHandler: handleNodeClick  // Add this line
          },
          position: { x: 0, y: 0 }
      };
		});

		const edges = data.edges.map((edge) => ({
			id: `${edge.source}-${edge.target}`,
			source: edge.source,
			target: edge.target,
			type: 'smoothstep',
			animated: true,
			style: edge.job_mrn ? 'stroke: #22c55e; stroke-width: 2px;' : 'stroke: #94a3b8;'
		}));

		const layoutedNodes = getLayoutedElements(nodes, edges);
		return { nodes: layoutedNodes, edges };
	}

	async function fetchLineage() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi(`/lineage/assets/${currentAsset.id}?depth=${depth}`);

			if (!response.ok) {
				throw new Error('Failed to fetch lineage');
			}

			const data = await response.json();

			// Only check node count and override depth on initial load
			if (initialLoad && data.nodes.length > 50 && depth > 1) {
				depth = 1;
				initialLoad = false;
				await fetchLineage();
				return;
			}

			initialLoad = false;
			const elements = generateElements(data);
			$nodes = elements.nodes;
			$edges = elements.edges;
		} catch (err) {
			console.error('Error fetching lineage:', err);
			error = err instanceof Error ? err.message : 'Failed to load lineage';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		mounted = true;
	});

	$: if (mounted && $page.url.searchParams.get('tab') === 'lineage') {
		fetchLineage();
	}
</script>

<div class="w-full h-[800px] relative">
	<!-- Depth Control - Always visible -->
	<div class="absolute right-4 top-4 z-[5] flex flex-col items-end gap-1">
		<span class="text-xs text-gray-600 dark:text-gray-400 px-1">Depth</span>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-lg overflow-hidden">
			<div class="flex flex-col items-center p-1">
				<Button
					variant="clear"
					text="+"
					class="!p-1"
					click={() => {
						depth = depth + 1;
						fetchLineage();
					}}
				/>
				<div class="py-1 text-sm text-gray-600 dark:text-gray-300">{depth}</div>
				<Button
					variant="clear"
					text="-"
					class="!p-1"
					disabled={depth <= 1}
					click={() => {
						depth = depth - 1;
						fetchLineage();
					}}
				/>
			</div>
		</div>
	</div>

	<!-- Graph container with its own loading state -->
	<div
		class="h-full bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
	>
		{#if loading}
			<div class="flex items-center justify-center h-full">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600" />
			</div>
		{:else if error}
			<div class="flex items-center justify-center h-full text-red-600 dark:text-red-400">
				{error}
			</div>
		{:else}
			<div class="h-full w-full">
				{#if $nodes}
					<SvelteFlow
						{nodes}
						{edges}
						{nodeTypes}
    nodeClick={handleNodeClick}
						fitView
						minZoom={0.2}
						maxZoom={1}
						initialZoom={0.7}
						defaultEdgeOptions={{
							type: 'smoothstep',
							animated: true,
							style: 'stroke-width: 2; stroke: #d1d5db;'
						}}
						nodesConnectable={false}
						elementsSelectable={true}
					>
						<Background gap={16} variant="dots" />
					</SvelteFlow>
				{/if}
			</div>
		{/if}
	</div>
</div>

{#if selectedAsset}
	<div class="fixed inset-0 flex items-end justify-end z-50 pointer-events-none">
		<div
			class="w-[32rem] border-l border-gray-200 dark:border-gray-700 overflow-auto pointer-events-auto"
		>
			<AssetBlade
				asset={selectedAsset}
				lineage={lineageData}
				onClose={() => (selectedAsset = null)}
			/>
		</div>
	</div>
{/if}

<style>
	:global(.svelte-flow) {
		background-color: #fefdf8 !important;
	}
	:global(.dark .svelte-flow) {
		background-color: #1a1a1a !important;
	}
</style>
