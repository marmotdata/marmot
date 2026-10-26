import { m } from '$lib/paraglide/messages';
import type { QueryDecision, QueryRecord, QueryRecordStatus } from '$lib/dataquery/types';

const RED = 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300';
const GREEN = 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300';
const AMBER = 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300';

export function decisionLabel(decision: QueryDecision): string {
	return decision === 'allowed' ? m.dataquery_decision_allowed() : m.dataquery_decision_denied();
}

export function statusLabel(status: QueryRecordStatus): string {
	switch (status) {
		case 'running':
			return m.dataquery_status_running();
		case 'completed':
			return m.dataquery_status_completed();
		case 'failed':
			return m.dataquery_status_failed();
		case 'denied':
			return m.dataquery_status_denied();
		default:
			return status;
	}
}

// A denied query only ever has the denied status, so one badge covers both fields
export function outcome(query: QueryRecord): { label: string; classes: string } {
	if (query.decision === 'denied') return { label: m.dataquery_decision_denied(), classes: RED };
	if (query.status === 'failed') return { label: statusLabel('failed'), classes: RED };
	if (query.status === 'completed') return { label: statusLabel('completed'), classes: GREEN };
	return { label: statusLabel(query.status), classes: AMBER };
}

export function decisionClasses(decision: QueryDecision): string {
	return decision === 'allowed' ? GREEN : RED;
}

export function clientLabel(client: string): string {
	if (client === 'mcp') return m.dataquery_client_mcp();
	if (client === 'api') return m.dataquery_client_api();
	if (client === 'cli') return m.dataquery_client_cli();
	return client;
}

export function subjectParts(subject: string): { kind: string; name: string } {
	const idx = subject.indexOf(':');
	if (idx === -1) return { kind: '', name: subject };
	const kind = subject.slice(0, idx);
	const name = subject.slice(idx + 1);
	if (kind === 'user') return { kind: m.dataquery_principal_user(), name };
	if (kind === 'service_account') return { kind: m.dataquery_principal_service_account(), name };
	return { kind, name };
}

// mrn://type/service/name maps onto the asset page route; source-level pseudo MRNs have no asset page
export function assetRouteParams(
	mrn: string
): { type: string; service: string; name: string } | null {
	if (!mrn.startsWith('mrn://')) return null;
	const parts = mrn.slice('mrn://'.length).split('/');
	if (parts.length < 3 || parts[0] === 'source') return null;
	const [type, service, ...rest] = parts;
	return { type, service, name: rest.join('/') };
}

// The API sends Go's zero time when an asset has never been synced
export function isRealTime(value?: string): value is string {
	if (!value) return false;
	const time = new Date(value).getTime();
	return !Number.isNaN(time) && time > 0;
}
