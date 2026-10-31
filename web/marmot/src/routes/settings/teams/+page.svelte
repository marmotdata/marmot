<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type {
		DataTableColumn,
		DataTableSort,
		DataTableSortChangeDetail
	} from '@marmotdata/elements/classes/data-table.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import type { MmPageChangeDetail } from '@marmotdata/elements/classes/pagination.js';
	import { setCrumbs } from '$lib/mm';
	import { fetchApi } from '$lib/api';
	import { handleApiError } from '$lib/stores/toast';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { m } from '$lib/paraglide/messages';
	import { providerName } from '$lib/users/format';
	import type { TeamRow } from '$lib/teams/types';
	import CreateTeamDrawer from '$components/team/CreateTeamDrawer.svelte';

	// The API has no team search, so every team is loaded a hundred at a time and searched, filtered and paged here
	const FETCH_LIMIT = 100;
	const PAGE_SIZE = 50;

	let canManage = $derived(holdsAtRoot($myAccess, 'teams:manage'));
	let canViewMembers = $derived(holdsAtRoot($myAccess, 'users:view'));
	let teamless = $state(0);

	let teams = $state<TeamRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let query = $state('');
	let source = $state<'all' | 'manual' | 'sso'>('all');
	let pageNumber = $state(1);
	let creating = $state(false);

	setCrumbs([
		{ label: m.settings_title(), href: resolve('/settings') },
		{ label: m.settings_teams() }
	]);

	let shown = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return teams
			.filter((t) =>
				source === 'all' ? true : source === 'sso' ? !!t.created_via_sso : !t.created_via_sso
			)
			.filter(
				(t) =>
					!q || t.name.toLowerCase().includes(q) || (t.description ?? '').toLowerCase().includes(q)
			);
	});
	// Sorting happens here, before paging, so it orders every team rather than only the visible page
	let sort = $state<DataTableSort>({ key: 'name', direction: 'ascending' });
	let sorted = $derived.by(() => {
		const col = columns.find((c) => c.key === sort.key);
		const read = (t: TeamRow) => (col?.value ? col.value(t) : t[sort.key as keyof TeamRow]) ?? '';
		const dir = sort.direction === 'ascending' ? 1 : -1;
		return [...shown].sort((a, b) => {
			const x = read(a);
			const y = read(b);
			return (
				(typeof x === 'number' && typeof y === 'number'
					? x - y
					: String(x).localeCompare(String(y))) * dir
			);
		});
	});
	let pageRows = $derived(sorted.slice((pageNumber - 1) * PAGE_SIZE, pageNumber * PAGE_SIZE));
	let filtered = $derived(!!query.trim() || source !== 'all');

	async function fetchTeams() {
		loading = true;
		error = '';
		try {
			const all: TeamRow[] = [];
			for (let offset = 0; ; offset += FETCH_LIMIT) {
				const response = await fetchApi(`/teams?limit=${FETCH_LIMIT}&offset=${offset}`);
				if (!response.ok) throw new Error(await handleApiError(response));
				const data = await response.json();
				const batch: TeamRow[] = data.teams ?? [];
				all.push(...batch);
				if (batch.length < FETCH_LIMIT || all.length >= (data.total ?? 0)) break;
			}
			teams = all;
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_load();
		} finally {
			loading = false;
		}
	}

	const columns: DataTableColumn<TeamRow>[] = [
		{
			key: 'name',
			header: m.teams_header_team(),
			sortable: true,
			width: 220,
			href: (t) => resolve('/settings/teams/[id]', { id: t.id })
		},
		{ key: 'description', header: m.common_description(), width: 360 },
		{
			key: 'member_count',
			header: m.teams_members_heading(),
			type: 'number',
			sortable: true,
			width: 110
		},
		{
			key: 'source',
			header: m.teams_col_membership_from(),
			sortable: true,
			width: 240,
			value: membershipFrom
		},
		{
			key: 'asset_count',
			header: m.teams_col_owned_assets(),
			type: 'number',
			sortable: true,
			width: 140
		},
		{
			key: 'webhook_count',
			header: m.teams_col_webhooks(),
			type: 'number',
			sortable: true,
			width: 110
		}
	];

	// Where a team's members come from: the identity provider groups mapped to it, the provider that created it, or people adding them by hand
	function membershipFrom(t: TeamRow): string {
		if (t.sso_groups?.length) {
			return t.sso_groups
				.map((g) =>
					m.teams_source_sso_group({ provider: providerName(g.provider), group: g.sso_group_name })
				)
				.join(', ');
		}
		return t.created_via_sso
			? m.teams_source_sso({ provider: providerName(t.sso_provider ?? '') })
			: m.teams_manual_badge();
	}

	async function countTeamless() {
		try {
			const res = await fetchApi('/users?team=none&status=active&limit=1');
			teamless = res.ok ? ((await res.json()).total ?? 0) : 0;
		} catch {
			teamless = 0;
		}
	}

	$effect(() => {
		if (canViewMembers) countTeamless();
	});

	onMount(() => {
		fetchTeams();
		// The old /teams/new route redirects here with ?create=1, so open the drawer and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			creating = true;
			goto(resolve('/settings/teams'), { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<svelte:head>
	<title>{m.settings_teams()} · {m.settings_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.settings_teams()}
		compact
		count={loading || error ? undefined : teams.length}
	>
		{#if canManage}
			<mm-button
				slot="actions"
				variant="primary"
				icon="group-add"
				onclick={() => (creating = true)}
			>
				{m.teams_create_team_button()}
			</mm-button>
		{/if}
	</mm-page-header>

	{#if teamless > 0}
		<mm-callout tone="info" heading={m.teams_teamless_heading()}>
			{m.teams_teamless_body()}
			<mm-link href={`${resolve('/settings/members')}?team=none`}>{m.teams_teamless_link()}</mm-link
			>
		</mm-callout>
	{/if}

	<div class="mm-cluster" style="align-items: end">
		<mm-search-field
			style="width: 20rem; max-width: 100%"
			label={m.teams_search_label()}
			hide-label
			placeholder={m.teams_search_by_name()}
			value={query}
			onmm-input={(e: CustomEvent<MmValueDetail>) => {
				query = e.detail.value;
				pageNumber = 1;
			}}
		></mm-search-field>
		<mm-select
			style="width: 14rem"
			label={m.teams_col_membership_from()}
			hide-label
			value={source}
			onmm-change={(e: CustomEvent<MmValueDetail>) => {
				source = e.detail.value as typeof source;
				pageNumber = 1;
			}}
		>
			<mm-option value="all" label={m.teams_any_source()}></mm-option>
			<mm-option value="manual" label={m.teams_manual_badge()}></mm-option>
			<mm-option value="sso" label={m.teams_single_sign_on()}></mm-option>
		</mm-select>
	</div>

	<mm-data-table
		label={m.settings_teams()}
		{columns}
		rows={pageRows}
		{sort}
		manual-sort
		onmm-sort-change={(e: CustomEvent<DataTableSortChangeDetail>) => {
			sort = { key: e.detail.key, direction: e.detail.direction ?? 'ascending' };
			pageNumber = 1;
		}}
		loading={loading && teams.length === 0}
		{error}
		empty-heading={filtered ? m.teams_no_teams_found() : m.teams_empty_title()}
		empty-text={filtered ? m.teams_no_match_body() : m.teams_empty_body()}
		onmm-retry={fetchTeams}
	></mm-data-table>

	{#if shown.length > PAGE_SIZE}
		<mm-pagination
			label={m.teams_pagination_label()}
			page={pageNumber}
			page-size={PAGE_SIZE}
			total={shown.length}
			onmm-page-change={(e: CustomEvent<MmPageChangeDetail>) => (pageNumber = e.detail.page)}
		></mm-pagination>
	{/if}
</div>

{#if creating}
	<CreateTeamDrawer
		onClose={() => (creating = false)}
		onCreated={(team) => {
			creating = false;
			goto(resolve('/settings/teams/[id]', { id: team.id }));
		}}
	/>
{/if}
