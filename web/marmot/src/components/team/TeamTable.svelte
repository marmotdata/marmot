<script lang="ts" module>
	import type { Team } from '$lib/teams/types';

	/** A team as the list endpoint returns it, with the SSO fields the shared type leaves out. */
	export type TeamRow = Team & { created_via_sso?: boolean; sso_provider?: string };
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { BTN_GHOST, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';

	let {
		teams,
		canManage = true,
		onDelete
	}: { teams: TeamRow[]; canManage?: boolean; onDelete: (teamId: string) => void } = $props();

	let deletingTeamId = $state<string | null>(null);
	let showDeleteModal = $state(false);
	let teamToDelete = $state<TeamRow | null>(null);

	async function handleDeleteTeam() {
		if (!teamToDelete) return;
		try {
			deletingTeamId = teamToDelete.id;
			const response = await fetchApi(`/teams/${teamToDelete.id}`, { method: 'DELETE' });
			if (response.ok) {
				toasts.success(m.teams_delete_success({ name: teamToDelete.name }));
				onDelete(teamToDelete.id);
				showDeleteModal = false;
				teamToDelete = null;
			} else {
				toasts.error(await handleApiError(response));
			}
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.teams_error_generic());
		} finally {
			deletingTeamId = null;
		}
	}

	function open(teamId: string) {
		goto(resolve('/settings/teams/[id]', { id: teamId }));
	}
</script>

<div class="overflow-x-auto">
	<table class={TABLE}>
		<thead class="border-b border-gray-100 dark:border-gray-700/60">
			<tr>
				<th class="{TH} pl-5">{m.teams_header_team()}</th>
				<th class={TH}>{m.common_description()}</th>
				<th class={TH}>{m.teams_header_source()}</th>
				<th class="{TH} pr-5 text-right"><span class="sr-only">{m.common_actions()}</span></th>
			</tr>
		</thead>
		<tbody class={TBODY}>
			{#each teams as team (team.id)}
				<tr class="{TR} cursor-pointer" onclick={() => open(team.id)}>
					<td class="{TD} pl-5">
						<a
							href={resolve('/settings/teams/[id]', { id: team.id })}
							class="flex items-center gap-2.5 font-medium text-gray-900 hover:underline dark:text-gray-100"
							onclick={(e) => e.stopPropagation()}
						>
							<IconifyIcon
								icon="material-symbols:groups-outline"
								class="h-4 w-4 flex-shrink-0 text-gray-400"
							/>
							{team.name}
						</a>
					</td>
					<td class="{TD} max-w-md truncate text-gray-500 dark:text-gray-400">
						{team.description || '—'}
					</td>
					<td class={TD}>
						{#if team.created_via_sso}
							<Pill tone="info">
								<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
								{m.teams_sso_provider_badge({ provider: team.sso_provider ?? '' })}
							</Pill>
						{:else}
							<Pill>{m.teams_manual_badge()}</Pill>
						{/if}
					</td>
					<td class="{TD} whitespace-nowrap pr-5 text-right">
						{#if team.created_via_sso}
							<span
								class="inline-flex px-2.5 text-gray-400"
								title={m.teams_sso_cannot_delete_title()}
							>
								<IconifyIcon icon="material-symbols:lock-outline" class="h-4 w-4" />
							</span>
						{:else if canManage}
							<button
								type="button"
								onclick={(e) => {
									e.stopPropagation();
									teamToDelete = team;
									showDeleteModal = true;
								}}
								disabled={deletingTeamId === team.id}
								class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
								title={m.teams_delete_tooltip()}
								aria-label={m.teams_delete_tooltip()}
							>
								<IconifyIcon icon="material-symbols:delete-outline" class="h-4 w-4" />
							</button>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.teams_delete_title()}
	message={m.teams_delete_message()}
	confirmText={m.common_delete()}
	resourceName={teamToDelete?.name || ''}
	requireConfirmation={true}
	onConfirm={handleDeleteTeam}
	onCancel={() => {
		showDeleteModal = false;
		teamToDelete = null;
	}}
/>
