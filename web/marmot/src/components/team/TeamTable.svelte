<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { Users, Lock, Trash2 } from 'lucide-svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import type { Team } from '$lib/teams/types';

	export let teams: Team[];
	export let totalTeams: number;
	export let offset: number;
	export let limit: number;

	const dispatch = createEventDispatcher();

	let deletingTeamId: string | null = null;
	let showDeleteModal = false;
	let teamToDelete: Team | null = null;

	async function handleDeleteTeam() {
		if (!teamToDelete) return;

		try {
			deletingTeamId = teamToDelete.id;
			const response = await fetchApi(`/teams/${teamToDelete.id}`, {
				method: 'DELETE'
			});

			if (response.ok) {
				toasts.success(m.teams_delete_success({ name: teamToDelete.name }));
				dispatch('delete', teamToDelete.id);
				showDeleteModal = false;
				teamToDelete = null;
			} else {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
			}
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.teams_error_generic());
		} finally {
			deletingTeamId = null;
		}
	}

	function goToTeamPage(teamId: string) {
		goto(resolve(`/teams/${teamId}`));
	}

	$: totalPages = Math.ceil(totalTeams / limit);
	$: currentPage = Math.floor(offset / limit) + 1;
</script>

<div class="overflow-x-auto">
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
				>
					{m.teams_header_team()}
				</th>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
				>
					{m.common_description()}
				</th>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
				>
					{m.teams_header_source()}
				</th>
				<th
					class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
				>
					{m.common_actions()}
				</th>
			</tr>
		</thead>
		<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
			{#each teams as team (team.id)}
				<tr
					class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 cursor-pointer transition-colors"
					on:click={() => goToTeamPage(team.id)}
				>
					<td class="px-6 py-4 whitespace-nowrap">
						<div class="flex items-center">
							<Users class="h-5 w-5 text-gray-400 mr-2" />
							<div>
								<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
									{team.name}
								</div>
							</div>
						</div>
					</td>
					<td class="px-6 py-4">
						<div class="text-sm text-gray-500 dark:text-gray-400 max-w-md truncate">
							{team.description || '—'}
						</div>
					</td>
					<td class="px-6 py-4 whitespace-nowrap">
						{#if team.created_via_sso}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
							>
								<Lock class="h-3 w-3 mr-1" />
								{m.teams_sso_provider_badge({ provider: team.sso_provider })}
							</span>
						{:else}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
							>
								{m.teams_manual_badge()}
							</span>
						{/if}
					</td>
					<td
						class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium"
						on:click|stopPropagation
					>
						{#if !team.created_via_sso}
							<button
								on:click={() => {
									teamToDelete = team;
									showDeleteModal = true;
								}}
								disabled={deletingTeamId === team.id}
								class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
								title={m.teams_delete_tooltip()}
							>
								<Trash2 class="h-4 w-4" />
							</button>
						{:else}
							<span
								class="text-gray-400 dark:text-gray-600"
								title={m.teams_sso_cannot_delete_title()}
							>
								<Lock class="h-4 w-4" />
							</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>

	{#if teams.length === 0}
		<div class="text-center py-8 text-gray-500 dark:text-gray-400">{m.teams_no_teams_found()}</div>
	{/if}

	{#if totalPages > 1}
		<div
			class="bg-white dark:bg-gray-900 px-4 py-3 flex items-center justify-between border-t border-gray-200 dark:border-gray-700"
		>
			<div class="flex-1 flex justify-between sm:hidden">
				<button
					on:click={() => dispatch('pageChange', Math.max(0, offset - limit))}
					disabled={currentPage === 1}
					class="relative inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{m.common_previous()}
				</button>
				<button
					on:click={() => dispatch('pageChange', offset + limit)}
					disabled={currentPage === totalPages}
					class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{m.common_next()}
				</button>
			</div>
			<div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
				<div>
					<p class="text-sm text-gray-700 dark:text-gray-300">
						{m.teams_pagination_showing({
							from: offset + 1,
							to: Math.min(offset + limit, totalTeams),
							total: totalTeams
						})}
					</p>
				</div>
				<div>
					<nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px">
						<button
							on:click={() => dispatch('pageChange', Math.max(0, offset - limit))}
							disabled={currentPage === 1}
							class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{m.common_previous()}
						</button>
						<button
							on:click={() => dispatch('pageChange', offset + limit)}
							disabled={currentPage === totalPages}
							class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{m.common_next()}
						</button>
					</nav>
				</div>
			</div>
		</div>
	{/if}
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.teams_delete_title()}
	message={m.teams_delete_confirm_message()}
	confirmText={m.common_delete()}
	resourceName={teamToDelete?.name || ''}
	requireConfirmation={true}
	onConfirm={handleDeleteTeam}
	onCancel={() => {
		showDeleteModal = false;
		teamToDelete = null;
	}}
/>
