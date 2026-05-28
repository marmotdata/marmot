<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount, untrack } from 'svelte';
	import { SvelteSet, SvelteMap } from 'svelte/reactivity';
	import type { Asset } from '$lib/assets/types';
	import type { LineageNode, LineageResponse } from '$lib/lineage/types';
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
	// Observed edges (e.g. AGENT_LOOKUP) default-on for agent-focused views,
	// off elsewhere so chatty agents don't pollute pipeline lineage graphs.
	let isAgentAsset = $derived(currentAsset.type?.toLowerCase() === 'agent');
	let showObserved = $state(false);

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

	function getNodeIconType(node: LineageNode): string {
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
		const graph = new SvelteMap<string, string[]>();
		const visited = new SvelteSet<string>();
		const recursionStack = new SvelteSet<string>();
		const backEdges = new SvelteSet<string>();

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

		// SvelteFlow children (nodes with parentId) are NOT laid out by dagre —
		// they keep the relative positions assigned at construction time. For
		// dagre-layout purposes, edges that touch a child get redirected to its
		// parent so the parent container's placement reflects all incoming
		// lineage of its members.
		const childToParent = new SvelteMap<string, string>();
		for (const n of nodeArray) {
			const parentId = (n as Node & { parentId?: string }).parentId;
			if (parentId) childToParent.set(n.id, parentId);
		}

		const topLevelNodes = nodeArray.filter((n) => !childToParent.has(n.id));

		topLevelNodes.forEach((node) => {
			let width: number;
			let height: number;
			if (node.type === 'agentClusterExpanded' && typeof node.style === 'string') {
				const wm = node.style.match(/width:\s*(\d+)px/);
				const hm = node.style.match(/height:\s*(\d+)px/);
				width = wm ? Number(wm[1]) : 320;
				height = hm ? Number(hm[1]) : 240;
			} else if (node.type === 'cycleReturn') {
				width = 120;
				height = 60;
			} else {
				const nodeName = (node.data as { name?: string })?.name || '';
				width = Math.max(180, nodeName.length * 10 + 60);
				height = 80;
			}
			g.setNode(node.id, { width, height });
		});

		edgeArray.forEach((edge) => {
			const src = childToParent.get(edge.source) ?? edge.source;
			const tgt = childToParent.get(edge.target) ?? edge.target;
			if (src === tgt) return; // intra-cluster edges don't shape outer layout
			g.setEdge(src, tgt);
		});

		dagre.layout(g);

		return nodeArray.map((node) => {
			if (childToParent.has(node.id)) {
				// Children keep their pre-computed grid positions inside the parent.
				return node;
			}
			const nodeWithPosition = g.node(node.id);
			if (!nodeWithPosition) return node;
			const baseX = nodeWithPosition.x - nodeWithPosition.width / 2;
			const baseY = nodeWithPosition.y - nodeWithPosition.height / 2;
			return {
				...node,
				position: { x: baseX, y: baseY }
			};
		});
	}

	function generateElements(data: LineageResponse) {
		const connections = new SvelteMap<string, { hasUpstream: boolean; hasDownstream: boolean }>();
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
		const cycleReturnNodes = new SvelteMap<string, Node>();
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
				if (edge.origin === 'observed' && !showObserved) {
					return;
				}
				const isObserved = edge.origin === 'observed';
				const stroke = isObserved
					? 'stroke: #607b60; stroke-width: 1.5px; stroke-dasharray: 5,4; opacity: 0.75;'
					: edge.job_mrn
						? 'stroke: #22c55e; stroke-width: 2px;'
						: 'stroke: #94a3b8;';
				modifiedEdges.push({
					id: `${edge.source}-${edge.target}`,
					source: edge.source,
					target: edge.target,
					type: 'custom',
					animated: true,
					style: stroke,
					// CustomEdge renders its own chip for observed edges.
					// Drop the arrowhead on observed edges — they aren't flow
					// relationships, they're "the agent looked here".
					...(isObserved ? { markerEnd: '' } : {}),
					data: {
						edgeId: edge.id,
						edgeType: edge.type,
						edgeOrigin: edge.origin,
						observationCount: edge.observation_count,
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

		// When observed edges are filtered out, drop any node that has no
		// surviving edge to the focal asset — otherwise nodes pulled in by
		// observed-only relationships render as orphans.
		const focalId = currentAsset.mrn || currentAsset.id;
		const reachable = new SvelteSet<string>([focalId]);
		modifiedEdges.forEach((edge) => {
			reachable.add(edge.source);
			reachable.add(edge.target);
		});

		const nodeArray = data.nodes
			.filter((node) => reachable.has(node.id))
			.map((node) => {
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

		// Agent-focal optimisation: when this asset is an agent, its upstream
		// neighbours (whether declared via the SDK or observed at runtime) can
		// number in the dozens. Group them by (provider, type) into cluster
		// cards so the graph stays scannable. Origin is preserved per-edge so
		// the cluster→agent style still distinguishes "declared dependency"
		// from "observed at runtime".
		const isAgentFocal = currentAsset.type?.toLowerCase() === 'agent';

		let finalNodes = nodeArray as Node[];
		let finalEdges = modifiedEdges;

		if (isAgentFocal) {
			const clustered = clusterAgentNeighbours(finalNodes, finalEdges, data, focalId);
			finalNodes = clustered.nodes;
			finalEdges = clustered.edges;
		}

		// Edge thickness scales sublinearly with observation_count so frequent
		// lookups stand out without dwarfing one-off declared edges.
		finalEdges = finalEdges.map((edge) => {
			const count = Number(edge.data?.observationCount ?? 1);
			if (count > 1 && edge.style && typeof edge.style === 'string') {
				const width = Math.min(6, 1.5 + Math.log2(count));
				edge.style =
					edge.style.replace(/stroke-width:\s*[\d.]+px;?/, '') +
					` stroke-width: ${width.toFixed(1)}px;`;
			}
			return edge;
		});

		const allNodes = [...finalNodes, ...Array.from(cycleReturnNodes.values())];
		const layoutedNodes = getLayoutedElements(allNodes, finalEdges);

		return { nodes: layoutedNodes, edges: finalEdges };
	}

	// Cluster a group only when there are multiple neighbours of the same
	// (provider, type). A lone postgres table or lone agent doesn't benefit
	// from being wrapped in a cluster card.
	const CLUSTER_THRESHOLD = 2;

	// Cluster keys (provider::type) the user has clicked to expand. Expanded
	// clusters render their member nodes individually instead of collapsing
	// into a single cluster card.
	let expandedClusters = new SvelteSet<string>();

	function toggleClusterExpansion(clusterKey: string) {
		if (expandedClusters.has(clusterKey)) expandedClusters.delete(clusterKey);
		else expandedClusters.add(clusterKey);
		if (lineageData) {
			const elements = generateElements(lineageData);
			nodes = elements.nodes;
			edges = elements.edges;
		}
	}

	function collapseAllClusters() {
		if (expandedClusters.size === 0) return;
		expandedClusters.clear();
		if (lineageData) {
			const elements = generateElements(lineageData);
			nodes = elements.nodes;
			edges = elements.edges;
		}
	}

	function clusterAgentNeighbours(
		nodeArray: Node[],
		edges: Edge[],
		data: LineageResponse,
		focalId: string
	): { nodes: Node[]; edges: Edge[] } {
		// Map nodeId → original asset (for provider/type lookup)
		const assetById = new SvelteMap<string, LineageResponse['nodes'][number]>();
		for (const n of data.nodes) assetById.set(n.id, n);

		// Walk every edge touching the focal agent. For each neighbour, track:
		//   - the max observation_count from any observed edge,
		//   - whether at least one declared edge touched it,
		//   - whether at least one observed edge touched it.
		// We keep all neighbours — declared, observed, or mixed. The styling
		// downstream uses the declared/observed flags to pick the cluster→agent
		// stroke.
		type Neighbour = {
			observationCount: number;
			hasDeclared: boolean;
			hasObserved: boolean;
		};
		const neighbours = new SvelteMap<string, Neighbour>();
		for (const edge of edges) {
			if (edge.source !== focalId && edge.target !== focalId) continue;
			const other = edge.source === focalId ? edge.target : edge.source;
			const cur: Neighbour = neighbours.get(other) ?? {
				observationCount: 0,
				hasDeclared: false,
				hasObserved: false
			};
			if (edge.data?.edgeOrigin === 'observed') {
				cur.hasObserved = true;
				const c = Number(edge.data?.observationCount ?? 1);
				cur.observationCount = Math.max(cur.observationCount, c);
			} else {
				cur.hasDeclared = true;
			}
			neighbours.set(other, cur);
		}

		// Group neighbours by (provider, type).
		const groups = new SvelteMap<string, string[]>();
		for (const [nodeId] of neighbours) {
			const asset = assetById.get(nodeId)?.asset;
			if (!asset) continue;
			const provider = asset.providers?.[0] || asset.type || 'asset';
			const type = asset.type || 'asset';
			const key = `${provider}::${type}`;
			(groups.get(key) ?? groups.set(key, []).get(key)!).push(nodeId);
		}

		const topLevelOut: Node[] = []; // cluster cards / containers
		const childOut: Node[] = []; // member nodes, when their cluster is expanded
		const clusterEdges: Edge[] = []; // synthetic cluster→focal edges
		const removedTopLevel = new SvelteSet<string>(); // member ids removed from top-level layout
		// Members of a COLLAPSED cluster — their incoming edges get redirected
		// onto the cluster card (so upstream chains terminate cleanly there
		// instead of leaving floating branches).
		const memberToCollapsedCluster = new SvelteMap<string, string>();
		// Members of an EXPANDED cluster — kept as individual nodes but reparented
		// inside the container. Their edges stay targeted at the member; dagre
		// just routes through the parent for layout.
		const expandedMemberSet = new SvelteSet<string>();

		for (const [key, memberIds] of groups) {
			if (memberIds.length < CLUSTER_THRESHOLD) continue;
			const [provider, type] = key.split('::');
			// Aggregate observation count + origin flags across the cluster so
			// the synthetic edge style can reflect whether this is a declared
			// dependency, an observed runtime access, or a mix of both.
			let totalObservations = 0;
			let clusterHasDeclared = false;
			let clusterHasObserved = false;
			for (const id of memberIds) {
				const n = neighbours.get(id);
				if (!n) continue;
				totalObservations += n.observationCount || (n.hasObserved ? 1 : 0);
				clusterHasDeclared ||= n.hasDeclared;
				clusterHasObserved ||= n.hasObserved;
			}
			const memberMRNs = memberIds
				.map((id) => assetById.get(id)?.asset?.mrn || id)
				.filter(Boolean) as string[];
			const clusterId = `cluster::${focalId}::${key}`;

			const originKind: 'declared' | 'observed' | 'mixed' =
				clusterHasDeclared && clusterHasObserved
					? 'mixed'
					: clusterHasDeclared
						? 'declared'
						: 'observed';

			if (expandedClusters.has(key)) {
				// Expanded → sized container with members in a grid. Each slot
				// is sized to the longest name in this cluster so long MRNs like
				// "GLACIER.FINANCE.FACT_SUBSCRIPTIONS" don't bleed past the
				// container edge (CustomNode auto-widens with name length).
				const maxNameLen = memberIds.reduce((max, id) => {
					const n = nodeArray.find((nd) => nd.id === id);
					const name = (n?.data as { name?: string })?.name ?? '';
					return Math.max(max, name.length);
				}, 0);
				const cols = Math.min(5, Math.max(3, Math.ceil(Math.sqrt(memberIds.length))));
				const rows = Math.ceil(memberIds.length / cols);
				// Match CustomNode's own width formula (max(180, len*10 + 60)),
				// add a small slack to keep things from kissing the gap.
				const memberW = Math.max(200, Math.min(420, maxNameLen * 10 + 60 + 16));
				const memberH = 175;
				const gap = 22;
				const headerH = 56;
				const padBottom = 20;
				const width = cols * memberW + (cols + 1) * gap;
				const height = headerH + rows * memberH + (rows + 1) * gap + padBottom;

				topLevelOut.push({
					id: clusterId,
					type: 'agentClusterExpanded',
					data: {
						provider,
						assetType: type,
						count: memberIds.length,
						totalObservations,
						originKind,
						clusterKey: key,
						onCollapse: toggleClusterExpansion
					},
					style: `width: ${width}px; height: ${height}px;`,
					// Push the container behind everything so edges entering it
					// from upstream nodes render OVER the frame instead of getting
					// occluded by it.
					zIndex: -1,
					selectable: false,
					draggable: false,
					position: { x: 0, y: 0 }
				} as Node);

				memberIds.forEach((id, idx) => {
					const original = nodeArray.find((n) => n.id === id);
					if (!original) return;
					const r = Math.floor(idx / cols);
					const c = idx % cols;
					const x = gap + c * (memberW + gap);
					const y = headerH + gap + r * (memberH + gap);
					childOut.push({
						...original,
						parentId: clusterId,
						extent: 'parent',
						zIndex: 1,
						position: { x, y }
					} as Node);
					expandedMemberSet.add(id);
					removedTopLevel.add(id);
				});
			} else {
				// Collapsed → small cluster card.
				topLevelOut.push({
					id: clusterId,
					type: 'agentCluster',
					data: {
						provider,
						assetType: type,
						count: memberIds.length,
						totalObservations,
						originKind,
						memberMRNs,
						clusterKey: key,
						onToggleExpand: toggleClusterExpansion
					},
					position: { x: 0, y: 0 }
				});

				memberIds.forEach((id) => {
					removedTopLevel.add(id);
					memberToCollapsedCluster.set(id, clusterId);
				});
			}

			// Single synthetic edge from cluster → focal, in both states. The
			// suppressLabel flag tells CustomEdge not to draw the redundant
			// "observed · Nx" chip — count is already on the cluster header.
			// Style mirrors origin: solid when the cluster contains any
			// declared edge; dashed when every member is observed-only.
			const clusterEdgeStyle = clusterHasDeclared
				? `stroke: #94a3b8; stroke-width: 1.5px;`
				: `stroke: #607b60; stroke-dasharray: 5,4; opacity: 0.85;`;
			clusterEdges.push({
				id: `${clusterId}-${focalId}`,
				source: clusterId,
				target: focalId,
				type: 'custom',
				animated: true,
				style: clusterEdgeStyle,
				...(clusterHasDeclared ? {} : { markerEnd: '' }),
				data: {
					edgeType: clusterHasDeclared ? 'DIRECT' : 'AGENT_LOOKUP',
					edgeOrigin: clusterHasObserved && !clusterHasDeclared ? 'observed' : 'declared',
					observationCount: totalObservations,
					suppressLabel: true
				}
			});
		}

		const keptTopLevel = nodeArray.filter((n) => !removedTopLevel.has(n.id));

		// Edge processing:
		//   - Drop any direct member↔focal edge — synthetic cluster→focal stands.
		//   - Drop edges fully inside a collapsed cluster (intra-cluster noise).
		//   - For collapsed-cluster members: redirect that endpoint onto the cluster.
		//   - For expanded-cluster members: keep targeting the member; SvelteFlow
		//     will route the edge to the child node inside the container.
		const seenRedirected = new SvelteSet<string>();
		const keptEdges: Edge[] = [];
		for (const e of edges) {
			const srcCol = memberToCollapsedCluster.get(e.source);
			const tgtCol = memberToCollapsedCluster.get(e.target);
			const srcExp = expandedMemberSet.has(e.source);
			const tgtExp = expandedMemberSet.has(e.target);

			// member ↔ focal
			if ((srcCol || srcExp) && e.target === focalId) continue;
			if ((tgtCol || tgtExp) && e.source === focalId) continue;

			// Both ends in collapsed clusters → drop
			if (srcCol && tgtCol) continue;

			let newSrc = e.source;
			let newTgt = e.target;
			if (srcCol) newSrc = srcCol;
			if (tgtCol) newTgt = tgtCol;

			if (newSrc === e.source && newTgt === e.target) {
				keptEdges.push(e);
				continue;
			}

			const dedupKey = `${newSrc}->${newTgt}`;
			if (seenRedirected.has(dedupKey)) continue;
			seenRedirected.add(dedupKey);

			keptEdges.push({
				...e,
				id: dedupKey,
				source: newSrc,
				target: newTgt
			});
		}

		return {
			nodes: [...keptTopLevel, ...topLevelOut, ...childOut],
			edges: [...keptEdges, ...clusterEdges]
		};
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
			lineageData = data;

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

	// When navigating between assets, reset the observed-toggle to the default
	// for the new focal asset (on for agents, off for everything else).
	$effect(() => {
		void currentAsset.id;
		untrack(() => {
			showObserved = isAgentAsset;
		});
	});

	// Re-render existing data when the observed-edge toggle flips, without
	// re-fetching from the server. Untrack node/edge reads + writes so the
	// effect only re-fires when showObserved actually changes — otherwise
	// reassigning nodes/edges would re-trigger the effect itself.
	$effect(() => {
		void showObserved;
		untrack(() => {
			if (lineageData && nodes.length > 0) {
				const elements = generateElements(lineageData);
				nodes = elements.nodes;
				edges = elements.edges;
			}
		});
	});

	let observedEdgeCount = $derived(
		(lineageData?.edges ?? []).filter((e) => e.origin === 'observed').length
	);

	let isFullscreen = $state(false);

	function toggleFullscreen() {
		isFullscreen = !isFullscreen;
	}

	$effect(() => {
		if (!isFullscreen) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') isFullscreen = false;
		};
		document.addEventListener('keydown', onKey);
		return () => document.removeEventListener('keydown', onKey);
	});
</script>

<SvelteFlowProvider>
	<div
		class="lineage-container w-full h-[800px] relative bg-earthy-brown-50 dark:bg-gray-900"
		class:lineage-fullscreen={isFullscreen}
	>
		<div class="absolute right-4 top-4 z-[5] flex flex-col items-end gap-2">
			{#if isFullscreen}
				<button
					onclick={toggleFullscreen}
					class="flex items-center gap-1.5 px-3 py-2 rounded-lg shadow-lg bg-earthy-terracotta-600 hover:bg-earthy-terracotta-700 text-white transition-colors"
					title="Exit fullscreen (Esc)"
					aria-label="Exit fullscreen"
				>
					<IconifyIcon icon="material-symbols:fullscreen-exit-rounded" class="w-4 h-4" />
					<span class="text-xs font-semibold">Exit fullscreen</span>
					<span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-white/20">Esc</span>
				</button>
			{:else}
				<button
					onclick={toggleFullscreen}
					class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-lg text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
					title="Enter fullscreen"
					aria-label="Enter fullscreen"
				>
					<IconifyIcon icon="material-symbols:fullscreen-rounded" class="w-4 h-4" />
				</button>
			{/if}
			<div class="flex flex-col items-end gap-1">
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

		{#if observedEdgeCount > 0}
			<label
				class="absolute left-4 top-24 z-[5] flex items-center gap-2 px-2.5 py-1.5 rounded-full border border-earthy-green-200 dark:border-earthy-green-800/50 bg-white dark:bg-gray-800 shadow-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors"
			>
				<input
					type="checkbox"
					bind:checked={showObserved}
					class="rounded border-gray-300 dark:border-gray-600 text-earthy-green-700 focus:ring-earthy-green-700"
				/>
				<IconifyIcon
					icon="material-symbols:visibility-outline"
					class="w-3.5 h-3.5 text-earthy-green-700 dark:text-earthy-green-500"
				/>
				<span
					class="text-[11px] font-semibold uppercase tracking-wider text-earthy-green-800 dark:text-earthy-green-300"
				>
					Observed
				</span>
				<span class="text-[11px] text-earthy-green-700 dark:text-earthy-green-500 font-mono">
					{observedEdgeCount}
				</span>
				<span
					class="w-3.5 h-3.5 inline-flex items-center justify-center rounded-full border border-gray-300 dark:border-gray-600 text-[10px] font-semibold text-gray-500 dark:text-gray-400 cursor-help"
					title="Runtime-observed access — captured from real execution, not declared data flow."
				>
					?
				</span>
			</label>
		{/if}

		{#if expandedClusters.size > 0}
			<button
				onclick={collapseAllClusters}
				class="absolute left-4 top-[calc(6rem+2.25rem)] z-[5] flex items-center gap-1.5 px-2.5 py-1.5 rounded-full border border-earthy-green-200 dark:border-earthy-green-800/50 bg-white dark:bg-gray-800 shadow-sm hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors text-[11px] font-semibold uppercase tracking-wider text-earthy-green-800 dark:text-earthy-green-300"
				title="Recollapse all expanded observed groups"
			>
				<IconifyIcon icon="material-symbols:unfold-less-rounded" class="w-3.5 h-3.5" />
				<span>Collapse groups</span>
				<span class="font-mono text-earthy-green-700 dark:text-earthy-green-500">
					{expandedClusters.size}
				</span>
			</button>
		{/if}

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

	/* In-browser fullscreen: pin the container to the viewport with a high
	   z-index so it floats above the rest of the page chrome. The terracotta
	   ring + animated entry make the mode shift unmistakable. */
	.lineage-fullscreen {
		position: fixed;
		inset: 0;
		width: 100vw !important;
		height: 100vh !important;
		z-index: 50;
		box-shadow:
			inset 0 0 0 3px #b65a3c,
			0 0 0 9999px rgba(0, 0, 0, 0.35);
		border-radius: 0 !important;
		animation: lineage-fullscreen-in 180ms ease-out;
	}

	@keyframes lineage-fullscreen-in {
		from {
			opacity: 0.6;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}
</style>
