import { m } from '$lib/paraglide/messages';
import type { Permission } from './types';

// Turns a role's permission keys into what a person needs to see: which parts of Marmot it reaches and how far, in words rather than resource_type:action pairs. OSS and Cloud spell resource types differently (assets and asset, service_accounts and serviceAccount), so both are matched by one normalised name.

export type AccessLevel = 'full' | 'read' | 'partial' | 'none';

export interface AreaAction {
	key: string;
	label: string;
	description: string;
	granted: boolean;
}

export interface AreaSummary {
	id: string;
	label: string;
	icon: string;
	level: AccessLevel;
	actions: AreaAction[];
}

export interface CategorySummary {
	id: string;
	label: string;
	areas: AreaSummary[];
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
	icon: string;
	category: string;
}

const areas: Record<string, AreaDef> = {
	asset: {
		label: m.roles_area_asset,
		icon: 'material-symbols:database-outline',
		category: 'catalog'
	},
	dataproduct: {
		label: m.roles_area_data_product,
		icon: 'material-symbols:inventory-2-outline',
		category: 'catalog'
	},
	glossary: {
		label: m.roles_area_glossary,
		icon: 'material-symbols:book-outline',
		category: 'catalog'
	},
	lineage: {
		label: m.roles_area_lineage,
		icon: 'material-symbols:account-tree-outline',
		category: 'catalog'
	},
	access: { label: m.roles_area_access, icon: 'material-symbols:key-outline', category: 'catalog' },
	user: { label: m.roles_area_user, icon: 'material-symbols:person-outline', category: 'identity' },
	team: { label: m.roles_area_team, icon: 'material-symbols:groups-outline', category: 'identity' },
	serviceaccount: {
		label: m.roles_area_service_account,
		icon: 'material-symbols:smart-toy-outline',
		category: 'identity'
	},
	role: { label: m.roles_area_role, icon: 'material-symbols:badge-outline', category: 'identity' },
	iam: {
		label: m.roles_area_iam,
		icon: 'material-symbols:shield-person-outline',
		category: 'identity'
	},
	sso: { label: m.roles_area_sso, icon: 'material-symbols:login', category: 'identity' },
	ingestion: {
		label: m.roles_area_ingestion,
		icon: 'material-symbols:play-circle-outline',
		category: 'platform'
	},
	plugin: {
		label: m.roles_area_plugin,
		icon: 'material-symbols:extension-outline',
		category: 'platform'
	},
	agent: {
		label: m.roles_area_agent,
		icon: 'material-symbols:smart-toy-outline',
		category: 'platform'
	},
	secretstore: {
		label: m.roles_area_secret_store,
		icon: 'material-symbols:lock-outline',
		category: 'platform'
	},
	metrics: { label: m.roles_area_metrics, icon: 'material-symbols:insights', category: 'platform' }
};

const categories: { id: string; label: () => string }[] = [
	{ id: 'catalog', label: m.roles_category_catalog },
	{ id: 'identity', label: m.roles_category_identity },
	{ id: 'platform', label: m.roles_category_platform }
];

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

function levelOf(actions: AreaAction[]): AccessLevel {
	const granted = actions.filter((a) => a.granted);
	if (granted.length === 0) return 'none';
	if (granted.length === actions.length) return 'full';
	if (
		granted.every((a) => a.key.endsWith(':view') || a.key.toLowerCase().endsWith(':viewneighbors'))
	)
		return 'read';
	return 'partial';
}

/** Every area the instance defines permissions for, with what the role grants in each. */
export function summariseRole(all: Permission[], grantedKeys: string[]): CategorySummary[] {
	const granted = new Set(grantedKeys);
	const byArea: Record<string, AreaAction[]> = {};
	for (const p of all) {
		const key = `${p.resource_type}:${p.action}`;
		(byArea[normaliseResource(p.resource_type)] ??= []).push({
			key,
			label: actionLabel(p.action),
			description: p.description,
			granted: granted.has(key)
		});
	}
	const summaries: Record<string, AreaSummary[]> = {};
	for (const [id, actions] of Object.entries(byArea)) {
		const def = areas[id];
		actions.sort((a, b) => {
			const ai = actionOrder.indexOf(a.key.split(':')[1].toLowerCase());
			const bi = actionOrder.indexOf(b.key.split(':')[1].toLowerCase());
			return (ai < 0 ? 99 : ai) - (bi < 0 ? 99 : bi);
		});
		(summaries[def?.category ?? 'platform'] ??= []).push({
			id,
			label: def?.label() ?? id,
			icon: def?.icon ?? 'material-symbols:tune',
			level: levelOf(actions),
			actions
		});
	}
	return categories
		.map((c) => ({
			id: c.id,
			label: c.label(),
			areas: (summaries[c.id] ?? []).sort((a, b) => a.label.localeCompare(b.label))
		}))
		.filter((c) => c.areas.length > 0);
}
