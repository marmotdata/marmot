<script lang="ts">
	import {
		SvelteFlow,
		Background,
		type Node,
		type Edge,
		type NodeTypes,
		useSvelteFlow
	} from '@xyflow/svelte';
	import CustomNode from './CustomNode.svelte';
	import CycleReturnNode from './CycleReturnNode.svelte';

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
		cycleReturn: CycleReturnNode
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
	onnodeclick={(event) => {
		if (event.node.type === 'custom') {
			handleNodeClick(event.node.id);
		}
	}}
	fitView
	minZoom={0.2}
	maxZoom={1}
	initialZoom={0.7}
	defaultEdgeOptions={{
		type: 'bezier',
		animated: true,
		style: 'stroke-width: 2; stroke: #d1d5db;'
	}}
	nodesConnectable={false}
	elementsSelectable={true}
>
	<Background gap={16} variant="dots" />
</SvelteFlow>