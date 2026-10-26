<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import KpiTile from '$components/ui/KpiTile.svelte';
	import Panel from '$components/ui/Panel.svelte';
	import Pill from '$components/ui/Pill.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import type { MyAccess } from '$lib/iam/myAccess';
	import { visibleItems, type SettingsHref } from '$lib/settings/sections';
	import { listServiceAccounts } from '$lib/serviceaccounts/api';
	import {
		attentionChecks,
		attentionCount,
		settingsAttention,
		type Attention,
		type AttentionTone
	} from '$lib/settings/attention';
	import { listRoles } from '$lib/roles/api';

	// What an administrator should look at first: headline counts and the short list of things that will break or already have. Each check runs on its own, so one failing or forbidden request only hides its own part.
	let { access }: { access: MyAccess } = $props();

	interface Kpi {
		id: string;
		label: string;
		href: SettingsHref;
		value: number | null;
		sub?: string;
		failed: boolean;
	}

	let items = $derived(visibleItems(access));
	let visible = $derived(new Set(items.map((i) => i.id)));

	let kpis = $state<Kpi[]>([]);
	let attention = $state<Attention[]>([]);
	let pendingChecks = $state(0);
	let failedChecks = $state(0);
	let started = false;

	async function total(path: string): Promise<number> {
		const res = await fetchApi(path);
		if (!res.ok) throw new Error(path);
		const body = await res.json();
		return body.total ?? 0;
	}

	function setKpi(id: string, patch: Partial<Kpi>) {
		kpis = kpis.map((k) => (k.id === id ? { ...k, ...patch } : k));
	}

	// A check adds its rows when it finishes; a failure is counted rather than shown per check, since the rest of the list still stands
	function check(run: () => Promise<Attention[]>) {
		pendingChecks++;
		run()
			.then((rows) => (attention = [...attention, ...rows]))
			.catch(() => failedChecks++)
			.finally(() => {
				pendingChecks--;
				// The overview has just looked, so the gear's count follows it rather than waiting out its own interval
				if (pendingChecks === 0) settingsAttention.set(attentionCount(attention));
			});
	}

	function start() {
		const wanted: Kpi[] = [];
		if (visible.has('members'))
			wanted.push({
				id: 'members',
				label: m.settings_members(),
				href: '/settings/members',
				value: null,
				failed: false
			});
		if (visible.has('teams'))
			wanted.push({
				id: 'teams',
				label: m.settings_teams(),
				href: '/settings/teams',
				value: null,
				failed: false
			});
		if (visible.has('service-accounts'))
			wanted.push({
				id: 'service-accounts',
				label: m.settings_service_accounts(),
				href: '/settings/service-accounts',
				value: null,
				failed: false
			});
		if (visible.has('roles'))
			wanted.push({
				id: 'roles',
				label: m.settings_roles(),
				href: '/settings/roles',
				value: null,
				failed: false
			});
		kpis = wanted;

		if (visible.has('members')) {
			total('/users')
				.then((value) => setKpi('members', { value }))
				.catch(() => setKpi('members', { failed: true }));
		}
		if (visible.has('teams')) {
			total('/teams?limit=1')
				.then((value) => setKpi('teams', { value }))
				.catch(() => setKpi('teams', { failed: true }));
		}
		if (visible.has('roles')) {
			listRoles()
				.then((roles) =>
					setKpi('roles', {
						value: roles.length,
						sub: m.settings_overview_kpi_custom({
							count: formatNumber(roles.filter((r) => !r.is_system).length)
						})
					})
				)
				.catch(() => setKpi('roles', { failed: true }));
		}

		// The service account list feeds both its tile and the key expiry check
		const accounts = visible.has('service-accounts') ? listServiceAccounts() : null;
		if (accounts) {
			accounts
				.then((list) =>
					setKpi('service-accounts', {
						value: list.length,
						sub: m.settings_overview_kpi_active({
							count: formatNumber(list.filter((a) => a.active).length)
						})
					})
				)
				.catch(() => setKpi('service-accounts', { failed: true }));
		}

		for (const run of attentionChecks(access, { accounts: accounts ?? undefined })) check(run);
	}

	// The checks wait until the caller's permissions have been read so they run once
	$effect(() => {
		if (access.checked && !started) {
			started = true;
			start();
		}
	});

	// Lapsed and failed things first, then what is about to break
	let sorted = $derived(
		[...attention].sort((a, b) => (a.tone === b.tone ? 0 : a.tone === 'danger' ? -1 : 1))
	);
	let hasChecks = $derived(attentionChecks(access).length > 0);

	const toneChip: Record<AttentionTone, string> = {
		warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
		danger: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
	};
</script>

{#if kpis.length > 0}
	<div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
		{#each kpis as kpi (kpi.id)}
			{#if kpi.value === null && !kpi.failed}
				<Skeleton class="h-[5.5rem]" />
			{:else}
				<KpiTile
					label={kpi.label}
					value={kpi.failed ? '–' : formatNumber(kpi.value ?? 0)}
					sub={kpi.failed ? m.settings_overview_kpi_unavailable() : kpi.sub}
					href={resolve(kpi.href)}
				/>
			{/if}
		{/each}
	</div>
{/if}

{#if hasChecks}
	<Panel
		flush
		title={m.settings_attention_heading()}
		description={m.settings_attention_description()}
		icon="material-symbols:notifications-outline"
		class={kpis.length > 0 ? 'mt-5' : ''}
	>
		{#if pendingChecks > 0 && sorted.length === 0}
			<div class="p-5"><Skeleton class="h-10" rows={3} /></div>
		{:else if sorted.length === 0}
			<EmptyState
				dashed={false}
				icon="material-symbols:check-circle-outline"
				title={failedChecks > 0
					? m.settings_attention_partial_title()
					: m.settings_attention_all_clear()}
				description={failedChecks > 0
					? m.settings_attention_partial_description()
					: m.settings_attention_all_clear_description()}
			/>
		{:else}
			<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
				{#each sorted as row (row.id)}
					<li>
						<a
							href={resolve(row.href as '/')}
							class="group flex items-center gap-3 px-5 py-3 transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40"
						>
							<span
								class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg {toneChip[
									row.tone
								]}"
							>
								<IconifyIcon icon={row.icon} class="h-4 w-4" />
							</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-sm text-gray-900 dark:text-gray-100">
									{row.title}
								</span>
								{#if row.detail}
									<span class="block truncate text-xs text-gray-500 dark:text-gray-400">
										{row.detail}
									</span>
								{/if}
							</span>
							<Pill tone={row.tone}>{row.pill}</Pill>
							<IconifyIcon
								icon="material-symbols:chevron-right"
								class="h-5 w-5 flex-shrink-0 text-gray-300 transition group-hover:text-earthy-terracotta-600"
							/>
						</a>
					</li>
				{/each}
			</ul>
			{#if failedChecks > 0}
				<p
					class="border-t border-gray-100 px-5 py-2.5 text-xs text-gray-500 dark:border-gray-700/60 dark:text-gray-400"
				>
					{m.settings_attention_partial_description()}
				</p>
			{/if}
		{/if}
	</Panel>
{/if}
