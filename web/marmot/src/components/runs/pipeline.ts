import cronstrue from 'cronstrue/i18n';
import { m } from '$lib/paraglide/messages';
import { getLocale } from '$lib/paraglide/runtime';
import { fetchApi } from '$lib/api';
import type { Run as StripRun } from '@marmotdata/elements/classes/run-strip.js';

// Shapes and helpers shared by the Plugins list, the pipeline page and the pipeline create and edit forms

export type ConfigValue =
	string | number | boolean | null | undefined | ConfigValue[] | { [key: string]: ConfigValue };

export type ConfigObject = Record<string, ConfigValue>;

export interface ConfigField {
	name: string;
	type: string;
	label: string;
	description: string;
	required: boolean;
	default?: unknown;
	options?: { label: string; value: string }[];
	sensitive: boolean;
	placeholder?: string;
	fields?: ConfigField[];
	is_array?: boolean;
	// The plugin names the section a field belongs to; ungrouped fields are the connection essentials
	group?: string;
	validation?: { pattern?: string; min?: number; max?: number };
	show_when?: { field: string; value: string };
	hidden?: boolean;
}

export interface PluginInfo {
	id: string;
	name: string;
	description?: string;
	icon?: string;
	category?: string;
	config_spec?: ConfigField[];
	supports_query?: boolean;
	features?: string[];
	// The kinds of asset the plugin documents and whether it finds lineage, read from its metadata
	catalogs?: string[];
	produces_lineage?: boolean;
}

// What a connection test learned: the plugin connected and finished, timed out or failed with an error
export interface ConnectionTestResult {
	succeeded: boolean;
	timed_out?: boolean;
	error?: string;
	duration_ms: number;
	assets: number;
	tables: number;
	lineage: number;
	asset_types?: Record<string, number>;
}

export interface ConnectionTestResponse {
	valid: boolean;
	errors?: { field: string; message: string }[];
	result?: ConnectionTestResult;
}

export interface Pipeline {
	id: string;
	name: string;
	plugin_id: string;
	config: ConfigObject;
	cron_expression: string;
	enabled: boolean;
	queryable?: boolean;
	ingest_on_query?: boolean;
	last_run_at?: string;
	last_run_status?: string;
	next_run_at?: string;
	managed_by?: string;
	description?: string;
	owner_type?: 'user' | 'team';
	owner_id?: string;
	owner_name?: string;
	asset_count?: number;
	created_by?: string;
	created_at: string;
	updated_at: string;
}

export type RunStatus = 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';

export interface JobRun {
	id: string;
	schedule_id?: string;
	plugin_run_id?: string;
	pipeline_name: string;
	source_name: string;
	run_id: string;
	status: RunStatus;
	claimed_by?: string;
	claimed_at?: string;
	started_at?: string;
	connecting_at?: string;
	extracting_at?: string;
	writing_at?: string;
	finished_at?: string;
	log?: string;
	error_message?: string;
	assets_created: number;
	assets_updated: number;
	assets_deleted: number;
	lineage_created?: number;
	documentation_added?: number;
	config?: ConfigObject;
	created_by: string;
	created_at: string;
	updated_at?: string;
}

// The base fields every plugin shares; the forms give them their own step
export const BASE_FIELDS = new Set(['tags', 'external_links', 'filter']);

// Plugin groups in the order the forms show them; any other group follows in the order the plugin uses it
export const GROUP_ORDER = ['Authentication', 'Discovery', 'Advanced', 'AI enrichment'];

// Groups that most people leave alone, so they start folded
export const FOLDED_GROUPS = new Set(['Advanced', 'AI enrichment']);

export const AWS_PLUGINS = ['s3', 'sns', 'sqs', 'dynamodb', 'kinesis'];

export function isRunning(status: string | undefined): boolean {
	return status === 'running' || status === 'claimed' || status === 'pending';
}

// Job statuses mapped onto the design system's run states
export function stripStatus(status: string | undefined): StripRun['status'] {
	switch (status) {
		case 'succeeded':
			return 'succeeded';
		case 'failed':
			return 'failed';
		case 'running':
		case 'claimed':
			return 'running';
		case 'pending':
			return 'queued';
		default:
			return 'skipped';
	}
}

export function pillState(
	status: string | undefined
): 'failed' | 'running' | 'queued' | 'cancelled' | 'succeeded' {
	switch (status) {
		case 'failed':
			return 'failed';
		case 'running':
		case 'claimed':
			return 'running';
		case 'pending':
			return 'queued';
		case 'cancelled':
			return 'cancelled';
		default:
			return 'succeeded';
	}
}

export function runDurationMs(run: Pick<JobRun, 'started_at' | 'finished_at'>): number | null {
	if (!run.started_at) return null;
	const end = run.finished_at ? new Date(run.finished_at).getTime() : Date.now();
	return Math.max(0, end - new Date(run.started_at).getTime());
}

export function toStripRun(run: JobRun): StripRun {
	return {
		id: run.id,
		status: stripStatus(run.status),
		startedAt: run.started_at ?? run.created_at,
		durationMs: runDurationMs(run) ?? undefined
	};
}

// "412 ms", "41 s", "3 min 12 s", "2 h 5 min", with non-breaking spaces before units
export function formatRunDuration(ms: number | null | undefined): string {
	if (ms === null || ms === undefined) return '';
	if (ms < 1_000) return `${Math.round(ms)}\u00a0ms`;
	const s = Math.round(ms / 1_000);
	if (s < 60) return `${s}\u00a0s`;
	const min = Math.floor(s / 60);
	if (min < 60) return s % 60 ? `${min}\u00a0min ${s % 60}\u00a0s` : `${min}\u00a0min`;
	const h = Math.floor(min / 60);
	return min % 60 ? `${h}\u00a0h ${min % 60}\u00a0min` : `${h}\u00a0h`;
}

// "27 Oct, 02:00", the way runs are named in the table and the drawer
export function formatStamp(iso: string): string {
	return new Date(iso).toLocaleString(getLocale(), {
		day: 'numeric',
		month: 'short',
		hour: '2-digit',
		minute: '2-digit'
	});
}

export function describeCron(cron: string | undefined): string {
	if (!cron) return m.runs_schedule_manual();
	try {
		return cronstrue.toString(cron, { locale: getLocale(), use24HourTimeFormat: true });
	} catch {
		return cron;
	}
}

// Who started a run, in plain words: the scheduler and Marmot itself are not people
export function startedBy(createdBy: string | undefined): string {
	if (!createdBy || createdBy === 'scheduler') return m.plugins_run_by_schedule();
	if (createdBy === 'system') return m.plugins_run_by_marmot();
	return createdBy;
}

export function initializeConfigDefaults(fields: ConfigField[], configObj: ConfigObject = {}) {
	for (const field of fields) {
		if (field.type === 'object' && field.is_array) {
			configObj[field.name] = [];
		} else if (field.type === 'object' && field.fields) {
			const nested: ConfigObject = {};
			configObj[field.name] = nested;
			initializeConfigDefaults(field.fields, nested);
		} else if (field.type === 'multiselect') {
			configObj[field.name] = [];
		} else if (field.default !== undefined && field.default !== null) {
			configObj[field.name] = field.default as ConfigValue;
		}
	}
	return configObj;
}

export function fillMissingDefaults(fields: ConfigField[], configObj: ConfigObject) {
	for (const field of fields) {
		if (field.type === 'object' && field.is_array) {
			if (!Array.isArray(configObj[field.name])) configObj[field.name] = [];
		} else if (field.type === 'object' && field.fields) {
			if (!configObj[field.name] || typeof configObj[field.name] !== 'object')
				configObj[field.name] = {};
			fillMissingDefaults(field.fields, configObj[field.name] as ConfigObject);
		} else if (field.type === 'multiselect') {
			if (!Array.isArray(configObj[field.name])) configObj[field.name] = [];
		} else if (
			field.default !== undefined &&
			field.default !== null &&
			configObj[field.name] === undefined
		) {
			configObj[field.name] = field.default as ConfigValue;
		}
	}
}

// Fields hidden by a show_when condition are not sent, so a plugin never sees settings for a mode it is not in
export function cleanConfigForSubmit(configObj: ConfigObject, fields: ConfigField[]): ConfigObject {
	const cleaned = { ...configObj };
	for (const field of fields) {
		if (field.show_when && cleaned[field.show_when.field] !== field.show_when.value) {
			delete cleaned[field.name];
		}
	}
	return cleaned;
}

export function shouldHideField(
	field: ConfigField,
	configObj: ConfigObject,
	rootConfig?: ConfigObject
): boolean {
	// A checked use_default sibling (AWS default credentials) makes the explicit fields beside it moot
	if (configObj.use_default === true && field.name !== 'use_default') return true;
	if (field.show_when) {
		const checkObj = rootConfig || configObj;
		if (checkObj[field.show_when.field] !== field.show_when.value) return true;
	}
	return false;
}

// Plugins that read files say where from with source_type, which follows from the paths people type
export function detectSourceType(spec: ConfigField[] | null | undefined, config: ConfigObject) {
	if (!spec?.some((f) => f.name === 'source_type')) return;
	let detected = 'local';
	for (const field of spec) {
		if (field.hidden || field.type !== 'string') continue;
		const val = config[field.name];
		if (typeof val !== 'string') continue;
		if (val.startsWith('s3://')) {
			detected = 's3';
			break;
		}
		if (val.startsWith('git::')) {
			detected = 'git';
			break;
		}
	}
	config.source_type = detected;
}

// Fields of a plugin's spec split into the ungrouped connection essentials and its named groups, in display order
export function groupFields(spec: ConfigField[] | null | undefined) {
	const fields = (spec ?? []).filter((f) => !BASE_FIELDS.has(f.name) && !f.hidden);
	const groups: { name: string; fields: ConfigField[] }[] = [];
	for (const field of fields) {
		if (!field.group) continue;
		let group = groups.find((g) => g.name === field.group);
		if (!group) groups.push((group = { name: field.group, fields: [] }));
		group.fields.push(field);
	}
	const rank = (name: string) => {
		const i = GROUP_ORDER.indexOf(name);
		return i === -1 ? GROUP_ORDER.length : i;
	};
	groups.sort((a, b) => rank(a.name) - rank(b.name));
	return { essentials: fields.filter((f) => !f.group), groups };
}

// Starts the plugin with these settings and reports what it could see, writing nothing; a pipeline being edited sends its id so masked secrets are tested with their stored values
export async function testConnection(
	pluginId: string,
	config: ConfigObject,
	scheduleId?: string
): Promise<ConnectionTestResponse> {
	const res = await fetchApi('/ingestion/test-connection', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ plugin_id: pluginId, config, schedule_id: scheduleId })
	});
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error || m.pipelines_test_error());
	}
	return res.json();
}

// "Database, Table and Column", in the reader's language; English follows the British style, without a serial comma
export function formatList(items: string[]): string {
	const locale = getLocale() === 'en' ? 'en-GB' : getLocale();
	try {
		return new Intl.ListFormat(locale, { type: 'conjunction' }).format(items);
	} catch {
		return items.join(', ');
	}
}
