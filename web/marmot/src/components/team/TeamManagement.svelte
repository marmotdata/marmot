<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import {
		BTN_PRIMARY,
		BTN_SECONDARY,
		LINK,
		SEARCH_FIELD,
		FILTER_PILL,
		FILTER_PILL_ON,
		FILTER_PILL_OFF
	} from '$lib/ui/classes';
	import Panel from '$components/ui/Panel.svelte';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';
	import EmptyState from '$components/ui/EmptyState.svelte';
	import TeamTable, { type TeamRow } from './TeamTable.svelte';
	import CreateTeamBlade from './CreateTeamBlade.svelte';

	// The teams list. The API has no team search, so a page of up to a hundred teams is filtered here and the server pages beyond that
	let {
		createOpen = $bindable(false),
		canManage = true
	}: { createOpen?: boolean; canManage?: boolean } = $props();

	type SourceFilter = 'all' | 'manual' | 'sso';

	const LIMIT = 100;

	let teams = $state<TeamRow[]>([]);
	let totalTeams = $state(0);
	let offset = $state(0);
	let teamQuery = $state('');
	let sourceFilter = $state<SourceFilter>('all');
	let loading = $state(true);
	let error = $state<string | null>(null);

	let hasSso = $derived(teams.some((t) => t.created_via_sso));
	let shown = $derived.by(() => {
		const q = teamQuery.trim().toLowerCase();
		return teams
			.filter((t) =>
				sourceFilter === 'all'
					? true
					: sourceFilter === 'sso'
						? t.created_via_sso
						: !t.created_via_sso
			)
			.filter(
				(t) =>
					!q || t.name.toLowerCase().includes(q) || (t.description ?? '').toLowerCase().includes(q)
			);
	});

	const PILL = FILTER_PILL;
	const PILL_ON = FILTER_PILL_ON;
	const PILL_OFF = FILTER_PILL_OFF;

	function handleCreated(team: { id: string }) {
		createOpen = false;
		fetchTeams();
		goto(resolve('/settings/teams/[id]', { id: team.id }));
	}

	async function fetchTeams() {
		try {
			loading = true;
			error = null;
			const params = new URLSearchParams({ limit: String(LIMIT), offset: String(offset) });
			const response = await fetchApi(`/teams?${params}`);
			const data = await response.json();
			teams = data.teams ?? [];
			totalTeams = data.total ?? teams.length;
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_generic();
		} finally {
			loading = false;
		}
	}

	function goToOffset(next: number) {
		offset = Math.max(0, next);
		fetchTeams();
	}

	async function handleTeamDeleted(teamId: string) {
		teams = teams.filter((t) => t.id !== teamId);
		await fetchTeams();
	}

	onMount(() => {
		fetchTeams();
		// The old /teams/new route redirects here with ?create=1, so open the blade and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			createOpen = true;
			goto(resolve('/settings/teams'), { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<div class="flex flex-wrap items-center gap-2" role="group" aria-label={m.teams_filter_aria()}>
		{#if hasSso}
			{#each [{ id: 'all', label: m.common_all() }, { id: 'manual', label: m.teams_manual_badge() }, { id: 'sso', label: m.teams_sso_badge() }] as option (option.id)}
				<button
					type="button"
					aria-pressed={sourceFilter === option.id}
					onclick={() => (sourceFilter = option.id as SourceFilter)}
					class="{PILL} {sourceFilter === option.id ? PILL_ON : PILL_OFF}">{option.label}</button
				>
			{/each}
		{/if}
	</div>
	<label class="{SEARCH_FIELD} w-full sm:w-72">
		<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
		<input
			type="search"
			bind:value={teamQuery}
			placeholder={m.teams_search_placeholder()}
			aria-label={m.teams_search_placeholder()}
			class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
		/>
	</label>
</div>

{#if error}
	<Alert tone="error">{error}</Alert>
{:else if loading && teams.length === 0}
	<Skeleton class="h-14" rows={4} />
{:else if teams.length === 0}
	<EmptyState
		icon="material-symbols:groups-outline"
		title={m.teams_empty_title()}
		description={m.teams_empty_body()}
	>
		{#snippet action()}
			{#if canManage}
				<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
					<IconifyIcon icon="material-symbols:group-add-outline" class="h-5 w-5" />
					{m.teams_new_button()}
				</button>
			{/if}
		{/snippet}
	</EmptyState>
{:else if shown.length === 0}
	<EmptyState
		icon="material-symbols:search-off"
		title={m.teams_no_teams_found()}
		description={m.teams_no_match_body()}
	>
		{#snippet action()}
			<button
				type="button"
				class="text-sm {LINK}"
				onclick={() => {
					teamQuery = '';
					sourceFilter = 'all';
				}}>{m.members_clear_filters()}</button
			>
		{/snippet}
	</EmptyState>
{:else}
	<Panel flush>
		<TeamTable teams={shown} {canManage} onDelete={handleTeamDeleted} />
		{#if totalTeams > LIMIT}
			<div
				class="flex items-center justify-between border-t border-gray-100 px-5 py-3 dark:border-gray-700/60"
			>
				<p class="text-xs tabular-nums text-gray-500 dark:text-gray-400">
					{m.teams_pagination_showing({
						from: offset + 1,
						to: Math.min(offset + teams.length, totalTeams),
						total: totalTeams
					})}
				</p>
				<div class="flex gap-2">
					<button
						type="button"
						class="{BTN_SECONDARY} px-2.5 py-1 text-xs"
						disabled={offset === 0 || loading}
						onclick={() => goToOffset(offset - LIMIT)}>{m.common_previous()}</button
					>
					<button
						type="button"
						class="{BTN_SECONDARY} px-2.5 py-1 text-xs"
						disabled={offset + teams.length >= totalTeams || loading}
						onclick={() => goToOffset(offset + LIMIT)}>{m.common_next()}</button
					>
				</div>
			</div>
		{/if}
	</Panel>
{/if}

{#if createOpen}
	<CreateTeamBlade onClose={() => (createOpen = false)} onCreated={handleCreated} />
{/if}
