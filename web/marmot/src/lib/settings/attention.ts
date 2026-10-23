import { writable } from 'svelte/store';
import { fetchApi } from '$lib/api';
import { m } from '$lib/paraglide/messages';
import { getLocale } from '$lib/paraglide/runtime';
import { holdsAtRoot, type MyAccess } from '$lib/iam/myAccess';
import { visibleItems } from '$lib/settings/sections';
import { listServiceAccounts, listAPIKeys } from '$lib/serviceaccounts/api';
import type { ServiceAccount } from '$lib/serviceaccounts/types';
import { listSSOProviders } from '$lib/sso/api';

// The things an administrator should look at: what has already broken and what is about to. The Settings overview lists them and the gear in the top bar counts them, so both read from these checks and cannot disagree.

const SOON_MS = 7 * 24 * 3_600_000;
// Reading each service account's keys is one request per account, so the check only runs where that stays cheap
const KEY_CHECK_LIMIT = 20;
// The gear re-checks at most this often, rather than on every navigation
const COUNT_TTL_MS = 5 * 60_000;

export type AttentionTone = 'warning' | 'danger';

export interface Attention {
	id: string;
	icon: string;
	tone: AttentionTone;
	title: string;
	detail?: string;
	pill: string;
	href: string;
}

export type AttentionCheck = () => Promise<Attention[]>;

function relative(iso: string): string {
	const diff = new Date(iso).getTime() - Date.now();
	const rtf = new Intl.RelativeTimeFormat(getLocale(), { numeric: 'auto' });
	const hours = Math.round(diff / 3_600_000);
	if (Math.abs(hours) < 24) return rtf.format(hours, 'hour');
	return rtf.format(Math.round(diff / 86_400_000), 'day');
}

async function keyRows(accounts: ServiceAccount[], now: number): Promise<Attention[]> {
	const active = accounts.filter((a) => a.active);
	if (active.length === 0 || active.length > KEY_CHECK_LIMIT) return [];
	const lists = await Promise.all(
		active.map((a) => listAPIKeys(a.id).then((keys) => ({ account: a, keys })))
	);
	const rows: Attention[] = [];
	for (const { account, keys } of lists) {
		for (const key of keys) {
			if (!key.expires_at) continue;
			const left = new Date(key.expires_at).getTime() - now;
			if (left > SOON_MS) continue;
			const expired = left <= 0;
			rows.push({
				id: `key-${key.id}`,
				icon: 'material-symbols:vpn-key-outline',
				tone: expired ? 'danger' : 'warning',
				title: expired
					? m.settings_attention_key_expired({ key: key.name, account: account.name })
					: m.settings_attention_key_expiring({ key: key.name, account: account.name }),
				detail: relative(key.expires_at),
				pill: expired ? m.settings_attention_pill_expired() : m.settings_attention_pill_expiring(),
				href: `/settings/service-accounts/${account.id}`
			});
		}
	}
	return rows;
}

async function pipelineRows(): Promise<Attention[]> {
	const res = await fetchApi('/ingestion/schedules?limit=200');
	if (!res.ok) throw new Error('schedules');
	const body: {
		schedules?: {
			id: string;
			name: string;
			last_run_status?: string;
			last_run_at?: string;
		}[];
	} = await res.json();
	return (body.schedules ?? [])
		.filter((p) => p.last_run_status === 'failed')
		.map((p): Attention => ({
			id: `pipeline-${p.id}`,
			icon: 'material-symbols:error-outline',
			tone: 'danger',
			title: m.settings_attention_pipeline_failed({ name: p.name }),
			detail: p.last_run_at
				? m.settings_attention_pipeline_failed_detail({ when: relative(p.last_run_at) })
				: undefined,
			pill: m.settings_attention_pill_failed(),
			href: '/runs'
		}));
}

async function ssoRows(): Promise<Attention[]> {
	const providers = await listSSOProviders();
	if (providers.length > 0) return [];
	return [
		{
			id: 'sso',
			icon: 'material-symbols:login',
			tone: 'warning',
			title: m.settings_attention_sso_missing(),
			detail: m.settings_attention_sso_missing_detail(),
			pill: m.settings_attention_pill_setup(),
			href: '/settings/authentication'
		}
	];
}

/** The checks the caller may run. Pass the service account list when it has already been asked for, so it is not fetched twice. */
export function attentionChecks(
	access: MyAccess,
	opts: { accounts?: Promise<ServiceAccount[]> } = {}
): AttentionCheck[] {
	const now = Date.now();
	const visible = new Set(visibleItems(access).map((i) => i.id));
	const checks: AttentionCheck[] = [];

	if (visible.has('service-accounts')) {
		checks.push(() => (opts.accounts ?? listServiceAccounts()).then((list) => keyRows(list, now)));
	}
	if (holdsAtRoot(access, 'ingestion:view') || holdsAtRoot(access, 'ingestion:manage')) {
		checks.push(pipelineRows);
	}
	if (visible.has('authentication')) {
		checks.push(ssoRows);
	}
	return checks;
}

/** How many critical things there are, which is what the gear's dot counts: what has already broken lights it, while what is only about to break stays on the overview. */
export function attentionCount(rows: Attention[]): number {
	return rows.filter((r) => r.tone === 'danger').length;
}

/** The count behind the dot on the Settings gear; 0 until checked and when nothing needs attention. */
export const settingsAttention = writable(0);

let checkedAt = 0;
let checkedFor = '';

/** Runs every check for the caller and updates the gear's count. A failed or forbidden check counts as nothing, since a dot for a check that cannot run would never clear. */
export async function refreshSettingsAttention(access: MyAccess, force = false): Promise<void> {
	const key = JSON.stringify([access.admin, [...access.root].sort()]);
	if (!force && key === checkedFor && Date.now() - checkedAt < COUNT_TTL_MS) return;
	checkedFor = key;
	checkedAt = Date.now();
	const results = await Promise.allSettled(attentionChecks(access).map((run) => run()));
	const rows = results.flatMap((r) => (r.status === 'fulfilled' ? r.value : []));
	settingsAttention.set(attentionCount(rows));
}
