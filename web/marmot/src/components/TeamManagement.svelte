<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import CreateTeamForm from './CreateTeamForm.svelte';
	import TeamTable from './TeamTable.svelte';

	let teams: any[] = [];
	let totalTeams = 0;
	let offset = 0;
	let limit = 10;
	let teamQuery = '';
	let creatingTeam = false;
	let loading = false;
	let error: string | null = null;
	let searchTimer: ReturnType<typeof setTimeout>;

	async function handleTeamCreated() {
		creatingTeam = false;
		await fetchTeams();
	}

	async function fetchTeams() {
		try {
			loading = true;
			const params = new URLSearchParams({
				limit: limit.toString(),
				offset: offset.toString()
			});

			const response = await fetchApi(`/teams?${params}`);
			const data = await response.json();
			teams = data.teams;
			totalTeams = data.total;
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	onMount(fetchTeams);

	$: {
		if (teamQuery !== undefined) {
			if (searchTimer) clearTimeout(searchTimer);
			searchTimer = setTimeout(() => {
				offset = 0;
				fetchTeams();
			}, 300);
		}
	}

	async function handleTeamDeleted(teamId: string) {
		teams = teams.filter((t) => t.id !== teamId);
		await fetchTeams();
	}
</script>

<div class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
	<div class="p-6">
		<div class="flex justify-between items-center mb-6">
			<div class="flex-1 max-w-md">
				<input
					type="text"
					placeholder="Search teams..."
					bind:value={teamQuery}
					class="w-full px-4 py-2 rounded-md border border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
			<button
				class="ml-4 px-4 py-2 bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={() => (creatingTeam = !creatingTeam)}
			>
				{creatingTeam ? 'Cancel' : 'Create Team'}
			</button>
		</div>

		{#if creatingTeam}
			<CreateTeamForm onTeamCreated={handleTeamCreated} />
		{/if}

		{#if loading && !teams.length}
			<div class="flex justify-center p-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700" />
			</div>
		{:else if error}
			<div class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
				{error}
			</div>
		{:else}
			<TeamTable
				{teams}
				{totalTeams}
				{offset}
				{limit}
				on:delete={(e) => handleTeamDeleted(e.detail)}
				on:refresh={fetchTeams}
				on:pageChange={(e) => {
					offset = e.detail;
					fetchTeams();
				}}
			/>
		{/if}
	</div>
</div>
