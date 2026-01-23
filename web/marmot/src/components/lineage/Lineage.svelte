<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import type { Asset } from '$lib/assets/types';
	import type { LineageResponse } from '$lib/lineage/types';
	import { page } from '$app/stores';
	import { SvelteFlowProvider, type Node, type Edge } from '@xyflow/svelte';
	import '@xyflow/svelte/dist/style.css';
	import dagre from '@dagrejs/dagre';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import FlowContent from '$components/ui/FlowContent.svelte';
	import IconifyIcon from '@iconify/svelte';
	import AddLineageModal from './AddLineageModal.svelte';
	import { auth } from '$lib/stores/auth';

	let { currentAsset }: { currentAsset: Asset } = $props();
	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));
	let selectedAsset: Asset | null = $state(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let mounted = false;
	let lineageData: LineageResponse | null = $state(null);
	let depth = $state(10);

	// Add lineage modal state
	let showAddLineageModal = $state(false);
	let clickedNodeMrn = $state('');
	let addLineageDirection = $state<'upstream' | 'downstream'>('upstream');
	let addLineagePosition = $state({ x: 0, y: 0 });

	// Delete lineage state
	let selectedEdgeId = $state<string | null>(null);
	let deletePosition = $state({ x: 0, y: 0 });
	let showDeleteButton = $state(false);

	let nodes = $state.raw<Node[]>([]);
	let edges = $state.raw<Edge[]>([]);

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

	$effect(() => {
		// Reset selected asset when page changes
		void $page;
		selectedAsset = null;
	});

	async function handleNodeClick(nodeId: string) {
		const clickedNode = nodes.find((n) => n.id === nodeId);
		const assetId = clickedNode?.data?.id || nodeId;

		// prevent node clicking for stub assets
		if (clickedNode?.data?.isStub) {
			return;
		}

		if (assetId && assetId !== currentAsset.id) {
			try {
				const encodedAssetId = encodeURIComponent(assetId);
				const [assetResponse, lineageResponse] = await Promise.all([
					fetchApi(`/assets/${encodedAssetId}`),
					fetchApi(`/lineage/assets/${encodedAssetId}`)
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

	function findBackEdges(edgeArray: Edge[]): Set<string> {
		const graph = new Map<string, string[]>();
		const visited = new Set<string>();
		const recursionStack = new Set<string>();
		const backEdges = new Set<string>();

		edgeArray.forEach((edge) => {
			if (!graph.has(edge.source)) graph.set(edge.source, []);
			graph.get(edge.source)!.push(edge.target);
		});

		function dfs(node: string): void {
			visited.add(node);
			recursionStack.add(node);

			const neighbors = graph.get(node) || [];
			for (const neighbor of neighbors) {
				if (recursionStack.has(neighbor)) {
					backEdges.add(`${node}-${neighbor}`);
				} else if (!visited.has(neighbor)) {
					dfs(neighbor);
				}
			}

			recursionStack.delete(node);
		}

		graph.forEach((_, nodeId) => {
			if (!visited.has(nodeId)) {
				dfs(nodeId);
			}
		});

		return backEdges;
	}

	function getLayoutedElements(nodeArray: Node[], edgeArray: Edge[]) {
		const g = new dagre.graphlib.Graph();
		g.setGraph({
			rankdir: 'LR',
			nodesep: 120,
			ranksep: 150,
			marginx: 50,
			marginy: 50
		});

		g.setDefaultEdgeLabel(() => ({}));

		nodeArray.forEach((node) => {
			const nodeName = (node.data as { name?: string })?.name || '';
			const dimensions = {
				width: node.type === 'cycleReturn' ? 120 : Math.max(180, nodeName.length * 10 + 60),
				height: node.type === 'cycleReturn' ? 60 : 80
			};
			g.setNode(node.id, dimensions);
		});

		edgeArray.forEach((edge) => {
			g.setEdge(edge.source, edge.target);
		});

		dagre.layout(g);

		return nodeArray.map((node) => {
			const nodeWithPosition = g.node(node.id);
			const baseX = nodeWithPosition.x - nodeWithPosition.width / 2;
			const baseY = nodeWithPosition.y - nodeWithPosition.height / 2;

			return {
				...node,
				position: { x: baseX, y: baseY }
			};
		});
	}

	function generateElements(data: LineageResponse) {
		const connections = new Map<string, { hasUpstream: boolean; hasDownstream: boolean }>();
		const edgeArray = data.edges || [];

		if (edgeArray.length === 0) {
			return {
				nodes: [
					{
						id: currentAsset.mrn || currentAsset.id,
						type: 'custom',
						data: {
							id: currentAsset.id,
							mrn: currentAsset.mrn || currentAsset.id,
							name: currentAsset.name || '',
							type: currentAsset.type,
							iconType: currentAsset.providers?.[0] || currentAsset.type,
							provider: currentAsset.provider,
							isCurrent: true,
							depth: 0,
							hasUpstream: false,
							hasDownstream: false,
							nodeClickHandler: handleNodeClick,
							...(canManageAssets && {
								onAddUpstream: handleAddUpstream,
								onAddDownstream: handleAddDownstream
							})
						},
						position: { x: 0, y: 0 }
					}
				],
				edges: []
			};
		}

		const backEdges = findBackEdges(edgeArray);
		const cycleReturnNodes = new Map<string, Node>();
		const modifiedEdges: Edge[] = [];

		edgeArray.forEach((edge) => {
			const edgeKey = `${edge.source}-${edge.target}`;
			if (backEdges.has(edgeKey)) {
				const cycleReturnId = `cycle-return-${edge.source}-${edge.target}`;

				if (!cycleReturnNodes.has(cycleReturnId)) {
					const targetNode = data.nodes.find((n) => n.id === edge.target);
					cycleReturnNodes.set(cycleReturnId, {
						id: cycleReturnId,
						type: 'cycleReturn',
						data: {
							targetId: edge.target,
							targetName: targetNode?.asset.name || 'Unknown',
							targetType: targetNode?.asset.type || 'unknown'
						},
						position: { x: 0, y: 0 }
					});
				}

				modifiedEdges.push({
					id: `${edge.source}-${cycleReturnId}`,
					source: edge.source,
					target: cycleReturnId,
					type: 'custom',
					animated: true,
					style: 'stroke: #f59e0b; stroke-width: 2px;',
					data: {
						edgeId: edge.id,
						...(canManageAssets && { onDelete: handleEdgeDelete })
					}
				});
			} else {
				modifiedEdges.push({
					id: `${edge.source}-${edge.target}`,
					source: edge.source,
					target: edge.target,
					type: 'custom',
					animated: true,
					style: edge.job_mrn ? 'stroke: #22c55e; stroke-width: 2px;' : 'stroke: #94a3b8;',
					data: {
						edgeId: edge.id,
						...(canManageAssets && { onDelete: handleEdgeDelete })
					}
				});
			}
		});

		modifiedEdges.forEach((edge) => {
			connections.set(edge.source, {
				hasUpstream: connections.get(edge.source)?.hasUpstream || false,
				hasDownstream: true
			});
			connections.set(edge.target, {
				hasUpstream: true,
				hasDownstream: connections.get(edge.target)?.hasDownstream || false
			});
		});

		const nodeArray = data.nodes.map((node) => {
			const nodeConnections = connections.get(node.id) || {
				hasUpstream: false,
				hasDownstream: false
			};
			return {
				id: node.id,
				type: 'custom',
				data: {
					id: node.asset.id,
					mrn: node.id,
					name: node.asset.name || '',
					type: node.asset.type,
					iconType: getNodeIconType(node),
					provider: node.asset.provider,
					isCurrent: node.id === currentAsset.mrn,
					depth: node.depth,
					hasUpstream: nodeConnections.hasUpstream,
					hasDownstream: nodeConnections.hasDownstream,
					isStub: node.asset.is_stub,
					nodeClickHandler: handleNodeClick,
					...(canManageAssets && {
						onAddUpstream: handleAddUpstream,
						onAddDownstream: handleAddDownstream
					})
				},
				position: { x: 0, y: 0 }
			};
		});

		const allNodes = [...nodeArray, ...Array.from(cycleReturnNodes.values())];
		const layoutedNodes = getLayoutedElements(allNodes, modifiedEdges);

		return { nodes: layoutedNodes, edges: modifiedEdges };
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

			const elements = generateElements(data);
			nodes = elements.nodes;
			edges = elements.edges;
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

	function handleAddUpstream(nodeMrn: string, event: MouseEvent) {
		// Clicking left button: adding an upstream dependency
		// The searched asset will be the SOURCE, clicked node is TARGET
		addLineageDirection = 'upstream';
		clickedNodeMrn = nodeMrn;

		// Position the search box near the button that was clicked
		const target = event.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		addLineagePosition = {
			x: rect.left - 400, // Position to the left of the button
			y: rect.top + rect.height / 2 - 50
		};

		showAddLineageModal = true;
	}

	function handleAddDownstream(nodeMrn: string, event: MouseEvent) {
		// Clicking right button: adding a downstream dependency
		// The clicked node will be the SOURCE, searched asset is TARGET
		addLineageDirection = 'downstream';
		clickedNodeMrn = nodeMrn;

		// Position the search box near the button that was clicked
		const target = event.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		addLineagePosition = {
			x: rect.right + 10, // Position to the right of the button
			y: rect.top + rect.height / 2 - 50
		};

		showAddLineageModal = true;
	}

	async function handleAddLineage(selectedAssetMrn: string) {
		try {
			let source: string;
			let target: string;

			if (addLineageDirection === 'upstream') {
				// Left button: searched asset flows INTO clicked node
				// SOURCE = searched asset, TARGET = clicked node
				source = selectedAssetMrn;
				target = clickedNodeMrn;
			} else {
				// Right button: clicked node flows INTO searched asset
				// SOURCE = clicked node, TARGET = searched asset
				source = clickedNodeMrn;
				target = selectedAssetMrn;
			}

			const response = await fetchApi('/lineage/direct', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ source, target })
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to create lineage');
			}

			// Refresh the lineage graph
			await fetchLineage();
		} catch (err) {
			console.error('Error creating lineage:', err);
			throw err;
		}
	}

	async function handleEdgeDelete(edgeId: string, position: { x: number; y: number }) {
		selectedEdgeId = edgeId;

		// Position delete confirmation near the bin icon
		deletePosition = {
			x: position.x - 100, // Center the dialog (width ~200px)
			y: position.y + 10 // Slightly below the bin icon
		};

		showDeleteButton = true;
	}

	async function handleDeleteLineage() {
		if (!selectedEdgeId) return;

		try {
			const response = await fetchApi(`/lineage/direct/${selectedEdgeId}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				throw new Error('Failed to delete lineage');
			}

			showDeleteButton = false;
			selectedEdgeId = null;

			// Refresh the lineage graph
			await fetchLineage();
		} catch (err) {
			console.error('Error deleting lineage:', err);
		}
	}

	function handleCancelDelete() {
		showDeleteButton = false;
		selectedEdgeId = null;
	}

	$effect(() => {
		if (mounted && $page.url.searchParams.get('tab') === 'lineage') {
			fetchLineage();
		}
	});
</script>

<SvelteFlowProvider>
	<div class="w-full h-[800px] relative">
		<div class="absolute right-4 top-4 z-[5] flex flex-col items-end gap-1">
			<span class="text-xs text-gray-600 dark:text-gray-400 px-1 select-none">Depth</span>
			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-lg overflow-hidden">
				<div class="flex flex-col items-center p-1">
					<button
						onclick={() => {
							depth = depth + 1;
						}}
						class="p-1 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
					>
						+
					</button>
					<div class="py-1 text-sm text-gray-600 dark:text-gray-300 select-none">{depth}</div>
					<button
						onclick={() => {
							depth = depth - 1;
						}}
						disabled={depth <= 1}
						class="p-1 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						-
					</button>
				</div>
			</div>
		</div>

		<div
			class="absolute left-4 top-4 z-[5] flex flex-col items-start gap-2 text-xs text-gray-600 dark:text-gray-400"
		>
			<div class="flex items-center gap-2">
				<IconifyIcon
					icon="bi:ticket-perforated-fill"
					class="w-3 h-3 text-earthy-terracotta-700 dark:text-earthy-terracotta-700 rotate-12"
				/>
				<span class="text-gray-600 dark:text-gray-300">Stub Asset</span>
			</div>
			<div class="flex items-center gap-1">
				<div class="w-4 h-0.5 bg-earthy-terracotta-600"></div>
				<span>Returns to</span>
			</div>
		</div>

		<div
			class="h-full bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-earthy-terracotta-100 dark:border-gray-700 relative"
		>
			{#if loading && (!nodes || nodes.length === 0)}
				<div class="absolute inset-0 flex items-center justify-center bg-inherit z-10">
					<div
						class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
					></div>
				</div>
			{/if}

			{#if error}
				<div class="absolute inset-0 flex items-center justify-center bg-inherit z-10">
					<div class="text-red-600 dark:text-red-400">{error}</div>
				</div>
			{/if}

			<div class="h-full w-full">
				{#if nodes && nodes.length > 0}
					<FlowContent
						{nodes}
						{edges}
						{handleNodeClick}
						onNodesChange={(updatedNodes) => {
							nodes = updatedNodes;
						}}
						onEdgesChange={(updatedEdges) => {
							edges = updatedEdges;
						}}
					/>
				{/if}
			</div>
		</div>
	</div>
</SvelteFlowProvider>

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

<AddLineageModal
	bind:show={showAddLineageModal}
	sourceMrn={clickedNodeMrn}
	targetMrn=""
	direction={addLineageDirection}
	position={addLineagePosition}
	onAdd={handleAddLineage}
/>

<!-- Delete Lineage Button -->
{#if showDeleteButton}
	<div
		class="fixed inset-0 z-[100]"
		onclick={handleCancelDelete}
		onkeydown={(e) => e.key === 'Escape' && handleCancelDelete()}
		role="button"
		tabindex="-1"
	></div>

	<div
		class="absolute z-[101] bg-white dark:bg-gray-800 rounded-lg shadow-2xl border border-gray-200 dark:border-gray-700 p-4"
		style="left: {deletePosition.x}px; top: {deletePosition.y}px;"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
		role="dialog"
		tabindex="-1"
	>
		<div class="flex items-center gap-3">
			<IconifyIcon
				icon="material-symbols:warning-rounded"
				class="w-6 h-6 text-red-600 dark:text-red-400"
			/>
			<div>
				<p class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
					Delete this lineage connection?
				</p>
				<div class="flex gap-2">
					<button
						onclick={handleCancelDelete}
						class="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
					>
						Cancel
					</button>
					<button
						onclick={handleDeleteLineage}
						class="px-3 py-1.5 text-sm text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 rounded-lg transition-colors"
					>
						Delete
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	:global(.svelte-flow) {
		background-color: #fefcfb !important;
	}
	:global(.dark .svelte-flow) {
		background-color: #1a1a1a !important;
	}
</style>
