import { m } from '$lib/paraglide/messages';
import type { Permission } from './types';

// Turns the instance's permissions into the grid the role editor shows: one row per part of Marmot, in words rather than resource_type, and one column per action. OSS and Cloud spell resource types differently (assets and asset, service_accounts and serviceAccount), so both are matched by one normalised name.

/** One cell of the grid: the permission it toggles, or none when the area does not offer that action. */
export interface MatrixRow {
	id: string;
	label: string;
	cells: Record<string, Permission | undefined>;
}

export interface MatrixColumn {
	id: string;
	label: string;
}

export interface PermissionMatrix {
	columns: MatrixColumn[];
	rows: MatrixRow[];
}

/** assets, Asset and asset are one area; so are service_accounts and serviceAccount. */
export function normaliseResource(resourceType: string): string {
	const flat = resourceType.toLowerCase().replace(/[_-]/g, '');
	return flat.endsWith('s') && !['access', 'sso', 'metrics'].includes(flat)
		? flat.slice(0, -1)
		: flat;
}

interface AreaDef {
	label: () => string;
	category: string;
}

const areas: Record<string, AreaDef> = {
	asset: {
		label: m.roles_area_asset,
		category: 'catalog'
	},
	dataproduct: {
		label: m.roles_area_data_product,
		category: 'catalog'
	},
	glossary: {
		label: m.roles_area_glossary,
		category: 'catalog'
	},
	lineage: {
		label: m.roles_area_lineage,
		category: 'catalog'
	},
	access: { label: m.roles_area_access, category: 'catalog' },
	user: { label: m.roles_area_user, category: 'identity' },
	team: { label: m.roles_area_team, category: 'identity' },
	serviceaccount: {
		label: m.roles_area_service_account,
		category: 'identity'
	},
	role: { label: m.roles_area_role, category: 'identity' },
	iam: {
		label: m.roles_area_iam,
		category: 'identity'
	},
	sso: { label: m.roles_area_sso, category: 'identity' },
	ingestion: {
		label: m.roles_area_ingestion,
		category: 'platform'
	},
	plugin: {
		label: m.roles_area_plugin,
		category: 'platform'
	},
	agent: {
		label: m.roles_area_agent,
		category: 'platform'
	},
	secretstore: {
		label: m.roles_area_secret_store,
		category: 'platform'
	},
	metrics: { label: m.roles_area_metrics, category: 'platform' }
};

// Rows run from the catalog to who can use it to the platform underneath
const categories = ['catalog', 'identity', 'platform'];

const actionLabels: Record<string, () => string> = {
	view: m.roles_action_view,
	manage: m.roles_action_manage,
	preview: m.roles_action_preview,
	query: m.roles_action_query,
	viewneighbors: m.roles_action_view_neighbors,
	emit: m.roles_action_emit,
	serve: m.roles_action_serve,
	publish: m.roles_action_publish,
	read: m.roles_action_read,
	use: m.roles_action_use
};

// Reading comes first, then what builds on it, then managing.
const actionOrder = [
	'view',
	'viewneighbors',
	'preview',
	'query',
	'read',
	'use',
	'emit',
	'serve',
	'publish',
	'manage'
];

function actionLabel(action: string): string {
	return actionLabels[action.toLowerCase()]?.() ?? action;
}

/** Every area the instance defines permissions for against every action any area offers, reading first and managing last. */
export function permissionMatrix(all: Permission[]): PermissionMatrix {
	const rows: Record<string, MatrixRow> = {};
	const actions = new Set<string>();
	for (const p of all) {
		const id = normaliseResource(p.resource_type);
		const action = p.action.toLowerCase();
		actions.add(action);
		rows[id] ??= { id, label: areas[id]?.label() ?? p.resource_type, cells: {} };
		rows[id].cells[action] = p;
	}
	const rank = (action: string) => {
		const i = actionOrder.indexOf(action);
		return i < 0 ? 99 : i;
	};
	const categoryRank = (id: string) => categories.indexOf(areas[id]?.category ?? 'platform');
	return {
		columns: [...actions]
			.sort((a, b) => rank(a) - rank(b))
			.map((id) => ({ id, label: actionLabel(id) })),
		rows: Object.values(rows).sort(
			(a, b) => categoryRank(a.id) - categoryRank(b.id) || a.label.localeCompare(b.label)
		)
	};
}
