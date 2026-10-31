import type {
	LineageEdgeKind,
	LineageEdgeOrigin,
	LineageGraph,
	LineageGraphEdge,
	LineageGraphNode
} from '@marmotdata/elements/classes/lineage-canvas.js';
import type { Asset } from '$lib/assets/types';
import type { LineageEdge, LineageResponse } from '$lib/lineage/types';

const KINDS = new Set<LineageEdgeKind>([
	'CONTAINS',
	'FEEDS',
	'PRODUCES',
	'REFERENCES',
	'AGENT_LOOKUP'
]);

// The canvas treats ids starting with "agents:" as synthetic groups and never offers to delete their edges, which is what grouped neighbours need since their edges stand for several stored edges.
export const GROUP_PREFIX = 'agents:group:';

export function isGroupId(id: string): boolean {
	return id.startsWith(GROUP_PREFIX);
}

/** Stored lineage calls plain data flow DIRECT; the canvas calls it FEEDS. */
export function edgeKind(type: string | undefined): LineageEdgeKind {
	const t = (type ?? '').toUpperCase() as LineageEdgeKind;
	return KINDS.has(t) ? t : 'FEEDS';
}

function edgeOrigin(e: LineageEdge): LineageEdgeOrigin {
	if (e.origin) return e.origin;
	return edgeKind(e.type) === 'AGENT_LOOKUP' ? 'observed' : 'declared';
}

/** The asset's first provider, which picks its logo. */
export function providerOf(asset: Pick<Asset, 'providers'> | null | undefined): string {
	return asset?.providers?.[0] ?? '';
}

/** The Discover route for an asset, from its MRN (`mrn://type/service/full.name`). */
export function assetHref(asset: Pick<Asset, 'mrn' | 'type'> | null | undefined): string {
	if (!asset?.mrn || !asset.type) return '';
	const parts = asset.mrn.replace('mrn://', '').split('/');
	if (parts.length < 3) return '';
	return `/discover/${asset.type.toLowerCase()}/${parts[1]}/${encodeURIComponent(parts.slice(2).join('/'))}`;
}

interface BuildOptions {
	/** Group the current asset's neighbours by provider and type, as an agent can touch dozens of tables. */
	groupNeighbours: boolean;
	/** Group ids the person has expanded. */
	expanded: ReadonlySet<string>;
	/** Names a group node, e.g. "12 tables". */
	groupName: (count: number, type: string) => string;
}

/** Turns the lineage API response into the canvas graph. */
export function buildGraph(
	data: LineageResponse | null,
	current: Asset,
	opts: BuildOptions
): LineageGraph {
	const currentId = current.mrn || current.id;
	const nodes = new Map<string, LineageGraphNode>();
	for (const n of data?.nodes ?? []) {
		nodes.set(n.id, {
			id: n.id,
			mrn: n.asset?.mrn || n.id,
			name: n.asset?.name || n.id.split('/').pop() || n.id,
			type: (n.asset?.type || n.type || '').toLowerCase(),
			provider: providerOf(n.asset),
			...(n.asset?.is_stub
				? { state: 'stub' as const }
				: n.last_run_status === 'failed'
					? { state: 'failed' as const }
					: {})
		});
	}
	if (!nodes.has(currentId)) {
		nodes.set(currentId, {
			id: currentId,
			mrn: current.mrn || current.id,
			name: current.name,
			type: (current.type || '').toLowerCase(),
			provider: providerOf(current)
		});
	}

	let edges: LineageGraphEdge[] = (data?.edges ?? []).map((e) => ({
		source: e.source,
		target: e.target,
		kind: edgeKind(e.type),
		origin: edgeOrigin(e),
		...(e.observation_count ? { observations: e.observation_count } : {}),
		...(e.last_seen_at ? { lastSeen: e.last_seen_at } : {}),
		...(e.job_mrn ? { job: e.job_mrn } : {})
	}));

	if (opts.groupNeighbours) {
		const groups = new Map<string, string[]>();
		const seen = new Set<string>();
		for (const e of edges) {
			if (e.source !== currentId && e.target !== currentId) continue;
			const other = e.source === currentId ? e.target : e.source;
			if (other === currentId || seen.has(other)) continue;
			const n = nodes.get(other);
			if (!n) continue;
			seen.add(other);
			const key = `${GROUP_PREFIX}${e.target === currentId ? 'up' : 'down'}:${n.provider}::${n.type}`;
			(groups.get(key) ?? groups.set(key, []).get(key)!).push(other);
		}

		const memberOf = new Map<string, string>();
		for (const [key, members] of groups) {
			if (members.length < 2) continue;
			if (opts.expanded.has(key)) continue;
			const first = nodes.get(members[0])!;
			for (const id of members) {
				memberOf.set(id, key);
				nodes.delete(id);
			}
			nodes.set(key, {
				id: key,
				mrn: '',
				name: opts.groupName(members.length, first.type),
				type: first.type,
				provider: first.provider
			});
		}

		if (memberOf.size) {
			// Edges touching a grouped member move onto the group, merged so one line stands for them all.
			const merged = new Map<string, LineageGraphEdge>();
			for (const e of edges) {
				const source = memberOf.get(e.source) ?? e.source;
				const target = memberOf.get(e.target) ?? e.target;
				if (source === target) continue;
				const k = `${source}\u0000${target}`;
				const prev = merged.get(k);
				if (!prev) {
					merged.set(k, { ...e, source, target });
					continue;
				}
				if (prev.origin !== e.origin) prev.origin = 'mixed';
				if (prev.kind !== e.kind) prev.kind = 'FEEDS';
				const obs = (prev.observations ?? 0) + (e.observations ?? 0);
				if (obs) prev.observations = obs;
				if (e.lastSeen && (!prev.lastSeen || e.lastSeen > prev.lastSeen))
					prev.lastSeen = e.lastSeen;
				if (prev.job !== e.job) delete prev.job;
			}
			edges = [...merged.values()];
		}
	}

	return { currentId, nodes: [...nodes.values()], edges };
}
