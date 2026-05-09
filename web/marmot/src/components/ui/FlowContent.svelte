<script lang="ts">
	import {
		SvelteFlow,
		Background,
		type Node,
		type Edge,
		type NodeTypes,
		type EdgeTypes,
		useSvelteFlow
	} from '@xyflow/svelte';
	import CustomNode from '$components/lineage/CustomNode.svelte';
	import CycleReturnNode from '$components/lineage/CycleReturnNode.svelte';
	import AgentClusterNode from '$components/lineage/AgentClusterNode.svelte';
	import AgentClusterExpandedNode from '$components/lineage/AgentClusterExpandedNode.svelte';
	import CustomEdge from '$components/lineage/CustomEdge.svelte';

	let {
		nodes,
		edges,
		handleNodeClick
	}: {
		nodes: Node[];
		edges: Edge[];
		handleNodeClick: (id: string) => void;
	} = $props();

	const { fitView } = useSvelteFlow();

	const nodeTypes: NodeTypes = {
		custom: CustomNode,
		cycleReturn: CycleReturnNode,
		agentCluster: AgentClusterNode,
		agentClusterExpanded: AgentClusterExpandedNode
	};

	const edgeTypes: EdgeTypes = {
		custom: CustomEdge
	};

	function handleReturnNodeClick(targetId: string) {
		const targetNode = nodes.find((n) => n.id === targetId);
		if (targetNode) {
			const targetNodes = nodes.filter((n) => n.id === targetId);
			fitView({
				nodes: targetNodes,
				padding: 0.3,
				duration: 800
			});
		}
	}

	// Only refit the view when the *focal* asset changes (initial load or
	// navigating to a different node). Cluster expand/collapse, depth tweaks,
	// drags etc. all leave the focal id unchanged, so the user's current pan/
	// zoom is preserved.
	let lastFocalId: string | null = null;
	$effect(() => {
		if (!nodes || nodes.length === 0) return;
		const focal = nodes.find((n) => n.data?.isCurrent === true);
		if (!focal) return;
		if (focal.id === lastFocalId) return;
		lastFocalId = focal.id;
		setTimeout(() => {
			fitView({
				nodes: [focal],
				padding: 0.5,
				duration: 800,
				maxZoom: 1
			});
		}, 100);
	});

	let processedNodes = $derived(
		nodes.map((node) => {
			if (node.type === 'cycleReturn') {
				return {
					...node,
					data: {
						...node.data,
						onReturnClick: handleReturnNodeClick
					}
				};
			}
			return node;
		})
	);
</script>

<SvelteFlow
	nodes={processedNodes}
	{edges}
	{nodeTypes}
	{edgeTypes}
	onnodeclick={(event) => {
		if (event.node.type === 'custom') {
			handleNodeClick(event.node.id);
		}
	}}
	minZoom={0.2}
	maxZoom={1}
	initialZoom={0.7}
	defaultEdgeOptions={{
		type: 'custom',
		animated: true,
		style: 'stroke-width: 2; stroke: #d1d5db;'
	}}
	nodesConnectable={false}
	elementsSelectable={true}
>
	<Background gap={16} variant="dots" />
</SvelteFlow>
