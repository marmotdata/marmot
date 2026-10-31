import { m } from '$lib/paraglide/messages';
import { getLocale } from '$lib/paraglide/runtime';
import { formatNumber } from '$lib/utils/format';
import type {
	QueryDecision,
	QueryRecord,
	QueryRecordStatus,
	QuerySourceTable
} from '$lib/dataquery/types';

// Soft status fills from the design tokens, so pages that still render a plain badge follow the theme
const RED = 'bg-mm-danger-soft text-mm-danger';
const GREEN = 'bg-mm-success-soft text-mm-success';
const AMBER = 'bg-mm-warning-soft text-mm-warning-text';

// The audit row, query history and status pill vocabulary: allowed is the default and is never badged
export type QueryOutcome = 'allowed' | 'denied' | 'failed' | 'running' | 'cancelled';

export function outcomeState(query: QueryRecord): QueryOutcome {
	if (query.decision === 'denied') return 'denied';
	if (query.status === 'failed') return 'failed';
	if (query.status === 'cancelled') return 'cancelled';
	if (query.status === 'running') return 'running';
	return 'allowed';
}

// The reason a query was refused or failed, or an empty string when it ran
export function outcomeReason(query: QueryRecord): string {
	if (query.decision === 'denied') return query.decision_detail?.reason ?? '';
	if (query.status === 'failed') return query.error ?? '';
	if (query.status === 'cancelled') return m.dataquery_reason_cancelled();
	return '';
}

// How the decision was reached, in words, such as "Allowed by role" or "Denied by policy"
export function outcomeReasonLabel(reason: string | undefined): string {
	switch (reason) {
		case 'role':
			return m.dataquery_reason_role();
		case 'grant':
			return m.dataquery_reason_grant();
		case 'permission':
			return m.dataquery_reason_permission();
		case 'policy':
			return m.dataquery_reason_policy();
		case 'source_error':
			return m.dataquery_reason_source_error();
		case 'timeout':
			return m.dataquery_reason_timeout();
		case 'unreachable':
			return m.dataquery_reason_unreachable();
		case 'cancelled':
			return m.dataquery_reason_cancelled();
		default:
			return reason ?? '';
	}
}

// The audit row names an agent by the client it identified itself as, with the service account or person it acted as alongside
export function actorName(query: QueryRecord): string {
	return query.agent || subjectParts(query.audit_subject).name;
}

export function actorServiceAccount(query: QueryRecord): string {
	if (query.service_account && query.service_account !== actorName(query))
		return query.service_account;
	if (query.agent) return subjectParts(query.audit_subject).name;
	return '';
}

// Table sizes read like the catalog's: exact below ten thousand, then 61.8K or 1.28M
export function formatRowCount(n: number): string {
	if (n < 10_000) return formatNumber(n);
	return new Intl.NumberFormat(getLocale(), {
		notation: 'compact',
		maximumSignificantDigits: 3
	}).format(n);
}

export function durationMs(query: QueryRecord): number | undefined {
	if (!query.completed_at || !isRealTime(query.completed_at)) return undefined;
	return new Date(query.completed_at).getTime() - new Date(query.started_at).getTime();
}

// Tables a query read, without the source-level pseudo MRN that stands for a whole source
export function tablesRead(query: QueryRecord): string[] {
	return (query.referenced_mrns ?? []).filter((mrn) => !mrn.startsWith('mrn://source/'));
}

// Service accounts are how agents call Marmot, so they read as agents with the robot avatar, as does anyone's key used by an agent that named itself
export function actorKind(query: QueryRecord): 'person' | 'agent' {
	return query.principal_type === 'service_account' || query.agent ? 'agent' : 'person';
}

// Opens a statement in the Query workspace from anywhere in the app
export function workspaceHref(source: string, statement: string, run = false): string {
	const params = new URLSearchParams({ source, q: statement });
	if (run) params.set('run', '1');
	return `/query?${params}`;
}

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
		case 'cancelled':
			return m.dataquery_status_cancelled();
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
	if (client === 'editor') return m.dataquery_client_editor();
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

// The sample statement's FROM clause carries the name as the source spells it, quoting and all
export function qualifiedName(table: QuerySourceTable): string {
	const from = table.sample_statement?.match(/FROM\s+(\S+)/i)?.[1];
	if (from) return from;
	return table.schema ? `${table.schema}.${table.name}` : table.name;
}

// Query times read "412 ms" or "1.8 s" with a non-breaking space before the unit
export function formatQueryDuration(ms: number): string {
	if (ms < 1000) return `${Math.round(ms)}\u00a0ms`;
	return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)}\u00a0s`;
}
