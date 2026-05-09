import type { Asset } from '$lib/assets/types';

export interface LineageNode {
	id: string;
	type: string;
	asset: Asset;
	depth: number;
}

export interface LineageEdge {
	id: string;
	source: string;
	target: string;
	type: string;
	origin?: 'declared' | 'observed';
	observation_count?: number;
	last_seen_at?: string;
	job_mrn?: string;
}

export interface LineageResponse {
	nodes: LineageNode[];
	edges: LineageEdge[];
}
