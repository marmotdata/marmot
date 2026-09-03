<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { auth } from '$lib/stores/auth';
	import { toasts, getErrorMessage } from '$lib/stores/toast';
	import { listGrants, createGrant, revokeGrant } from '$lib/gateway/api';
	import type { GatewayGrant } from '$lib/gateway/types';

	interface Props {
		principalType: string;
		principalId: string;
	}

	let { principalType, principalId }: Props = $props();

	let canManage = $derived(auth.hasPermission('gateway', 'manage'));

	let grants = $state<GatewayGrant[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showCreate = $state(false);
	let creating = $state(false);
	let newSelector = $state('');
	let newReason = $state('');
	let newExpiryDays = $state<number | null>(null);

	let showRevoke = $state(false);
	let revokeRow = $state<GatewayGrant | null>(null);

	async function load() {
		loading = true;
		error = '';
		try {
			grants = await listGrants(principalType, principalId);
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function submitCreate() {
		creating = true;
		try {
			let expiresAt: string | undefined;
			if (newExpiryDays && newExpiryDays > 0) {
				expiresAt = new Date(Date.now() + newExpiryDays * 24 * 60 * 60 * 1000).toISOString();
			}
			await createGrant({
				principal_type: principalType,
				principal_id: principalId,
				resource_selector: newSelector.trim(),
				expires_at: expiresAt,
				reason: newReason.trim() || undefined
			});
			toasts.success('Grant created');
			showCreate = false;
			newSelector = '';
			newReason = '';
			newExpiryDays = null;
			await load();
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			creating = false;
		}
	}

	async function confirmRevoke() {
		if (!revokeRow) return;
		try {
			await revokeGrant(revokeRow.id);
			toasts.success('Grant revoked — queries it covered are refused from now on');
			await load();
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			showRevoke = false;
			revokeRow = null;
		}
	}

	function grantState(g: GatewayGrant): { label: string; classes: string } {
		if (g.revoked_at) {
			return {
				label: 'revoked',
				classes: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
			};
		}
		if (g.expires_at && new Date(g.expires_at) < new Date()) {
			return {
				label: 'expired',
				classes: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
			};
		}
		return {
			label: 'live',
			classes: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
		};
	}

	onMount(load);
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
>
	<div class="flex items-start justify-between mb-4">
		<h2 class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center">
			<IconifyIcon
				icon="material-symbols:vpn-key-outline"
				class="h-5 w-5 mr-2 text-earthy-terracotta-600"
			/>
			Query grants
		</h2>
		{#if canManage}
			<Button
				variant="clear"
				icon="material-symbols:add"
				text="Add grant"
				click={() => (showCreate = !showCreate)}
			/>
		{/if}
	</div>
	<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
		What this principal may query through the gateway. Deny by default: a query runs only when every
		table it touches matches a live grant.
	</p>

	{#if showCreate}
		<div
			class="mb-4 p-4 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40 space-y-3"
		>
			<div>
				<label
					for="grant-selector"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
					>Resource selector (MRN glob)</label
				>
				<input
					id="grant-selector"
					type="text"
					bind:value={newSelector}
					placeholder="mrn://*/postgresql/**"
					class="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
				/>
				<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
					<code>**</code> spans path segments, <code>*</code> stays within one. Example:
					<code>mrn://table/postgresql/orders</code> for a single table.
				</p>
			</div>
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
				<div>
					<label
						for="grant-expiry"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
						>Expires in (days, optional)</label
					>
					<input
						id="grant-expiry"
						type="number"
						min="1"
						bind:value={newExpiryDays}
						class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
					/>
				</div>
				<div>
					<label
						for="grant-reason"
						class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
						>Reason (optional)</label
					>
					<input
						id="grant-reason"
						type="text"
						bind:value={newReason}
						placeholder="Why this access exists"
						class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600"
					/>
				</div>
			</div>
			<div class="flex justify-end gap-3">
				<Button variant="clear" text="Cancel" click={() => (showCreate = false)} />
				<Button
					variant="filled"
					icon="material-symbols:check"
					text={creating ? 'Creating...' : 'Create grant'}
					disabled={creating || !newSelector.trim()}
					click={submitCreate}
				/>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-6">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
	{:else if grants.length === 0}
		<p class="text-sm text-gray-500 dark:text-gray-400">
			No grants. This principal's gateway queries are all denied.
		</p>
	{:else}
		<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
			{#each grants as grant (grant.id)}
				{@const state = grantState(grant)}
				<li class="py-3 flex items-center justify-between gap-4">
					<div class="min-w-0">
						<code class="text-sm text-gray-900 dark:text-gray-100 break-all"
							>{grant.resource_selector}</code
						>
						<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
							{#if grant.reason}{grant.reason} · {/if}
							created {new Date(grant.created_at).toLocaleDateString()}
							{#if grant.expires_at}
								· expires {new Date(grant.expires_at).toLocaleDateString()}
							{/if}
						</div>
					</div>
					<div class="flex items-center gap-3 shrink-0">
						<span class="px-2 py-0.5 text-xs rounded-full {state.classes}">{state.label}</span>
						{#if canManage && state.label === 'live'}
							<button
								onclick={() => {
									revokeRow = grant;
									showRevoke = true;
								}}
								class="text-xs font-medium text-red-600 dark:text-red-400 hover:underline"
							>
								Revoke
							</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<ConfirmModal
	bind:show={showRevoke}
	title="Revoke grant"
	message={`Revoke access to ${revokeRow?.resource_selector}? Queries it covered are denied from the next attempt, with the denial audited.`}
	confirmText="Revoke"
	onConfirm={confirmRevoke}
	onCancel={() => (revokeRow = null)}
/>
