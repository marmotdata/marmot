<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { Users, Lock, Trash2 } from 'lucide-svelte';

	export let teams: any[];
	export let totalTeams: number;
	export let offset: number;
	export let limit: number;

	const dispatch = createEventDispatcher();

	let deletingTeamId: string | null = null;

	async function deleteTeam(teamId: string) {
		if (!confirm('Are you sure you want to delete this team?')) {
			return;
		}

		try {
			deletingTeamId = teamId;
			const response = await fetchApi(`/teams/${teamId}`, {
				method: 'DELETE'
			});

			if (response.ok) {
				dispatch('delete', teamId);
			} else {
				const data = await response.json();
				alert(data.error || 'Failed to delete team');
			}
		} catch (err: any) {
			alert(err.message);
		} finally {
			deletingTeamId = null;
		}
	}

	import { goto } from '$app/navigation';

	function goToTeamPage(teamId: string) {
		goto(`/teams/${teamId}`);
	}

	$: totalPages = Math.ceil(totalTeams / limit);
	$: currentPage = Math.floor(offset / limit) + 1;
</script>

<div class="overflow-x-auto">
	<table class="min-w-full">
		<thead>
			<tr>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800">
					Team
				</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800">
					Description
				</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800">
					Source
				</th>
				<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800">
					Actions
				</th>
			</tr>
		</thead>
		<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
			{#each teams as team (team.id)}
				<tr class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 cursor-pointer transition-colors" on:click={() => goToTeamPage(team.id)}>
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
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
								<Lock class="h-3 w-3 mr-1" />
								SSO ({team.sso_provider})
							</span>
						{:else}
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
								Manual
							</span>
						{/if}
					</td>
					<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium" on:click|stopPropagation>
						{#if !team.created_via_sso}
							<button
								on:click={() => deleteTeam(team.id)}
								disabled={deletingTeamId === team.id}
								class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
								title="Delete team"
							>
								<Trash2 class="h-4 w-4" />
							</button>
						{:else}
							<span class="text-gray-400 dark:text-gray-600" title="SSO-managed teams cannot be deleted">
								<Lock class="h-4 w-4" />
							</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>

	{#if teams.length === 0}
		<div class="text-center py-8 text-gray-500 dark:text-gray-400">
			No teams found
		</div>
	{/if}

	{#if totalPages > 1}
		<div class="bg-white dark:bg-gray-900 px-4 py-3 flex items-center justify-between border-t border-gray-200 dark:border-gray-700">
			<div class="flex-1 flex justify-between sm:hidden">
				<button
					on:click={() => dispatch('pageChange', Math.max(0, offset - limit))}
					disabled={currentPage === 1}
					class="relative inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Previous
				</button>
				<button
					on:click={() => dispatch('pageChange', offset + limit)}
					disabled={currentPage === totalPages}
					class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					Next
				</button>
			</div>
			<div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
				<div>
					<p class="text-sm text-gray-700 dark:text-gray-300">
						Showing <span class="font-medium">{offset + 1}</span> to
						<span class="font-medium">{Math.min(offset + limit, totalTeams)}</span> of
						<span class="font-medium">{totalTeams}</span> teams
					</p>
				</div>
				<div>
					<nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px">
						<button
							on:click={() => dispatch('pageChange', Math.max(0, offset - limit))}
							disabled={currentPage === 1}
							class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Previous
						</button>
						<button
							on:click={() => dispatch('pageChange', offset + limit)}
							disabled={currentPage === totalPages}
							class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm font-medium text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Next
						</button>
					</nav>
				</div>
			</div>
		</div>
	{/if}
</div>
