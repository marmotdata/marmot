import type { Asset } from '$lib/assets/types';

export interface LineageNode {
	id: string;
	type: string;
	asset: Asset;
	depth: number;
	/** Outcome of the latest run recorded against the asset. */
	last_run_status?: 'succeeded' | 'failed' | 'running';
}

export interface ColumnEdge {
	from_columns: string[];
	to_column: string;
	transform?: string;
	confidence?: number;
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
	/** The pipeline, person or service account that declared the edge. */
	declared_by?: string;
	column_lineage?: ColumnEdge[];
}

export interface LineageResponse {
	nodes: LineageNode[];
	edges: LineageEdge[];
}
