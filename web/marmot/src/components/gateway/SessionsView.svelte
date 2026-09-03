<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { auth } from '$lib/stores/auth';
	import { toasts, getErrorMessage } from '$lib/stores/toast';
	import { listSessions, revokeSession } from '$lib/gateway/api';
	import type { GatewaySession } from '$lib/gateway/types';

	let canManage = $derived(auth.hasPermission('gateway', 'manage'));

	let sessions = $state<GatewaySession[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showRevoke = $state(false);
	let revokeRow = $state<GatewaySession | null>(null);

	async function load() {
		loading = true;
		error = '';
		try {
			sessions = await listSessions(100, 0);
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	function sessionState(s: GatewaySession): { label: string; classes: string } {
		if (s.revoked_at) {
			return {
				label: 'revoked',
				classes: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
			};
		}
		if (new Date(s.expires_at) < new Date()) {
			return {
				label: 'expired',
				classes: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
			};
		}
		return {
			label: 'active',
			classes: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
		};
	}

	async function confirmRevoke() {
		if (!revokeRow) return;
		try {
			await revokeSession(revokeRow.id);
			toasts.success('Session revoked — its next query will be refused');
			await load();
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			showRevoke = false;
			revokeRow = null;
		}
	}

	function formatTime(value?: string): string {
		return value ? new Date(value).toLocaleString() : '—';
	}

	onMount(load);
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Agent sessions</h2>
		<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
			What agents hold instead of database credentials. Revoking one takes effect on its very next
			query.
		</p>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4">
			<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
		</div>
	{:else if sessions.length === 0}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-dashed border-gray-300 dark:border-gray-600 p-10 text-center"
		>
			<IconifyIcon
				icon="material-symbols:badge-outline"
				class="h-10 w-10 mx-auto text-gray-400 dark:text-gray-500"
			/>
			<p class="mt-3 text-sm text-gray-600 dark:text-gray-400">
				No sessions yet. Agents open one with their service account API key.
			</p>
		</div>
	{:else}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
		>
			<div class="overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr
							class="text-left text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
						>
							<th class="px-4 py-3 font-medium">Session</th>
							<th class="px-4 py-3 font-medium">Principal</th>
							<th class="px-4 py-3 font-medium">Purpose</th>
							<th class="px-4 py-3 font-medium">Opened</th>
							<th class="px-4 py-3 font-medium">Last activity</th>
							<th class="px-4 py-3 font-medium">State</th>
							{#if canManage}
								<th class="px-4 py-3"></th>
							{/if}
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-700/60">
						{#each sessions as session (session.id)}
							{@const state = sessionState(session)}
							<tr class="text-gray-700 dark:text-gray-300">
								<td class="px-4 py-3 font-mono text-xs">{session.id.slice(0, 8)}</td>
								<td class="px-4 py-3 font-mono text-xs"
									>{session.principal_type}:{session.principal_id.slice(0, 8)}</td
								>
								<td class="px-4 py-3">{session.purpose || '—'}</td>
								<td class="px-4 py-3 whitespace-nowrap">{formatTime(session.created_at)}</td>
								<td class="px-4 py-3 whitespace-nowrap">{formatTime(session.last_activity_at)}</td>
								<td class="px-4 py-3">
									<span class="px-2 py-0.5 text-xs rounded-full {state.classes}">{state.label}</span>
								</td>
								{#if canManage}
									<td class="px-4 py-3 text-right">
										{#if state.label === 'active'}
											<button
												onclick={() => {
													revokeRow = session;
													showRevoke = true;
												}}
												class="text-xs font-medium text-red-600 dark:text-red-400 hover:underline"
											>
												Revoke
											</button>
										{/if}
									</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

<ConfirmModal
	bind:show={showRevoke}
	title="Revoke session"
	message="The agent's next query on this session dies immediately with an audited denial. This cannot be undone."
	confirmText="Revoke"
	onConfirm={confirmRevoke}
	onCancel={() => (revokeRow = null)}
/>
