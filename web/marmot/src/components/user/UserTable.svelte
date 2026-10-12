<script lang="ts">
	import { onMount } from 'svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import EditUserForm from './EditUserForm.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import { KeyRound, Lock, Mail, Pencil, RotateCcw, Trash2 } from 'lucide-svelte';

	export let users = [];
	export let editingUserId = null;
	export let onEdit;
	export let onUpdate;
	export let onDelete;

	let totpStatus: 'loading' | 'disabled' | 'ready' | 'error' = 'loading';
	let totpPolicy: 'loading' | 'off' | 'optional' | 'required' | 'error' = 'loading';
	let totpUsers = new Set<string>();
	let resetUser: { id: string; username: string } | null = null;
	let passwordResetUser: { id: string; username: string } | null = null;
	let resetting = false;
	onMount(async () => {
		try {
			const response = await fetch('/auth-providers');
			if (!response.ok) throw new Error('Auth configuration unavailable');
			const config = await response.json();
			totpPolicy = !config.totp_enabled ? 'off' : config.totp_required ? 'required' : 'optional';
			if (totpPolicy === 'off') {
				totpStatus = 'disabled';
				return;
			}
			const enrolled = await fetchApi('/users/totp/enrolled');
			if (!enrolled.ok) throw new Error('TOTP enrollment unavailable');
			totpUsers = new Set((await enrolled.json()).user_ids);
			totpStatus = 'ready';
		} catch {
			if (totpPolicy === 'loading') totpPolicy = 'error';
			totpStatus = 'error';
		}
	});
	async function resetTOTP() {
		if (!resetUser || resetting) return;
		resetting = true;
		try {
			const response = await fetchApi(`/users/totp/reset/${encodeURIComponent(resetUser.id)}`, {
				method: 'DELETE'
			});
			if (!response.ok) throw new Error(m.totp_error());
			toasts.success(m.totp_reset_done());
			totpUsers = new Set([...totpUsers].filter((id) => id !== resetUser?.id));
			resetUser = null;
		} catch {
			toasts.error(m.totp_error());
		} finally {
			resetting = false;
		}
	}

	async function requirePasswordChange() {
		if (!passwordResetUser) return;
		const target = passwordResetUser;
		try {
			const response = await fetchApi(
				`/users/password/require-change/${encodeURIComponent(target.id)}`,
				{ method: 'POST' }
			);
			if (!response.ok) throw new Error(m.users_password_reset_error());
			const updated = users.find((item) => item.id === target.id);
			if (updated) onUpdate({ ...updated, must_change_password: true });
			toasts.success(m.users_password_reset_done());
			passwordResetUser = null;
		} catch {
			toasts.error(m.users_password_reset_error());
		}
	}

	let showDeleteModal = false;
	let userToDelete = null;

	const currentUserId = auth.getCurrentUserId();

	async function handleDelete() {
		if (!userToDelete) return;

		try {
			const response = await fetchApi(`/users/${userToDelete.id}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				return;
			}
			toasts.success(m.users_delete_success({ name: userToDelete.username }));
			onDelete(userToDelete.id);
			showDeleteModal = false;
			userToDelete = null;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_delete());
		}
	}

	function getProviderDisplay(provider: string): string {
		const providerMap: Record<string, string> = {
			google: 'Google',
			github: 'GitHub',
			gitlab: 'GitLab',
			okta: 'Okta',
			slack: 'Slack',
			auth0: 'Auth0'
		};
		return providerMap[provider] || provider.charAt(0).toUpperCase() + provider.slice(1);
	}
</script>

<div class="mb-4 flex items-center justify-end gap-2 text-xs" aria-live="polite">
	<span class="text-gray-500 dark:text-gray-400">{m.users_totp_policy_label()}</span>
	<span
		class={`inline-flex items-center rounded-full border px-2.5 py-1 font-medium ${totpPolicy === 'required' ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900 dark:text-green-200' : totpPolicy === 'optional' ? 'border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200' : 'border-gray-200 bg-gray-100 text-gray-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300'}`}
	>
		{totpPolicy === 'required'
			? m.users_totp_policy_required()
			: totpPolicy === 'optional'
				? m.users_totp_policy_optional()
				: totpPolicy === 'off'
					? m.users_totp_policy_off()
					: totpPolicy === 'loading'
						? m.common_loading()
						: m.totp_status_unavailable()}
	</span>
</div>

<div class="overflow-x-auto">
	<table class="min-w-full">
		<thead>
			<tr>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.users_username_label()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_name()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.users_header_auth()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.users_header_roles()}</th
				>
				<th
					class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_status()}</th
				>
				<th
					class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider bg-earthy-brown-100 dark:bg-gray-800"
					>{m.common_actions()}</th
				>
			</tr>
		</thead>
		<tbody class="divide-y divide-earthy-brown-100 bg-earthy-brown-50 dark:bg-gray-900">
			{#each users as user (user.id)}
				<tr class="hover:bg-earthy-brown-100 dark:hover:bg-gray-800 transition-colors">
					{#if editingUserId === user.id}
						<td colspan="6">
							<EditUserForm
								{user}
								onCancel={() => onEdit(null)}
								onUpdate={(updatedUser) => onUpdate(updatedUser)}
							/>
						</td>
					{:else}
						<td
							class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100"
							>{user.username}</td
						>
						<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400"
							>{user.name}</td
						>
						<td class="px-6 py-4 whitespace-nowrap">
							<div class="flex flex-wrap items-center gap-1">
								{#if user.identities && user.identities.length > 0}
									{#each user.identities as identity (identity.provider)}
										<span
											class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
										>
											<Lock class="h-3 w-3 mr-1" />
											{getProviderDisplay(identity.provider)}
										</span>
									{/each}
								{:else}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
									>
										<Mail class="h-3 w-3 mr-1" />
										{m.users_auth_password_badge()}
									</span>
								{/if}
								{#if !user.identities?.length}
									<span
										class={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${totpStatus === 'ready' && totpUsers.has(user.id) ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}`}
										>{totpStatus === 'ready'
											? totpUsers.has(user.id)
												? m.totp_badge()
												: m.totp_not_enabled()
											: totpStatus === 'disabled'
												? m.totp_server_disabled()
												: m.totp_status_unavailable()}</span
									>
								{/if}
							</div>
						</td>
						<td class="px-6 py-4 whitespace-nowrap">
							<div class="flex flex-wrap gap-1">
								{#each user.roles as role (role.name)}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
										>{role.name}</span
									>
								{/each}
							</div>
						</td>
						<td class="px-6 py-4 whitespace-nowrap">
							<span
								class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${user.active ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'}`}
							>
								{user.active ? m.common_active() : m.common_inactive()}
							</span>
							{#if user.must_change_password}<span
									class="ml-2 text-xs text-amber-700 dark:text-amber-300"
									>{m.users_password_reset_pending()}</span
								>{/if}
						</td>
						<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
							<div class="flex items-center justify-end gap-1">
								{#if totpStatus === 'ready' && totpUsers.has(user.id) && currentUserId !== user.id && (auth.hasRole('admin') || auth.hasPermission('users', 'manage'))}
									<button
										type="button"
										class="rounded-md p-2 text-earthy-terracotta-700 hover:bg-earthy-terracotta-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-earthy-terracotta-600 dark:text-earthy-terracotta-300 dark:hover:bg-gray-700"
										aria-label={`${m.totp_reset()}: ${user.username}`}
										title={m.totp_reset()}
										on:click={() => (resetUser = user)}><RotateCcw class="h-4 w-4" /></button
									>
								{/if}
								{#if currentUserId !== user.id}
									{#if !user.identities?.length && (auth.hasRole('admin') || auth.hasPermission('users', 'manage'))}
										<button
											type="button"
											class="rounded-md p-2 text-earthy-terracotta-700 hover:bg-earthy-terracotta-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-earthy-terracotta-600 dark:text-earthy-terracotta-300 dark:hover:bg-gray-700"
											aria-label={`${m.users_require_password_change()}: ${user.username}`}
											title={m.users_require_password_change()}
											on:click={() => (passwordResetUser = user)}
											><KeyRound class="h-4 w-4" /></button
										>
									{/if}
									<button
										type="button"
										class="rounded-md p-2 text-earthy-terracotta-700 hover:bg-earthy-terracotta-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-earthy-terracotta-600 dark:text-earthy-terracotta-300 dark:hover:bg-gray-700"
										aria-label={`${m.common_edit()}: ${user.username}`}
										title={m.common_edit()}
										on:click={() => onEdit(user.id)}
									>
										<Pencil class="h-4 w-4" />
									</button>
								{/if}
								{#if currentUserId !== user.id && user.username !== 'admin'}
									<button
										type="button"
										class="rounded-md p-2 text-red-600 hover:bg-red-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-red-600 dark:text-red-400 dark:hover:bg-red-950"
										aria-label={`${m.common_delete()}: ${user.username}`}
										title={m.common_delete()}
										on:click={() => {
											userToDelete = user;
											showDeleteModal = true;
										}}
									>
										<Trash2 class="h-4 w-4" />
									</button>
								{/if}
							</div>
						</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<DeleteModal
	show={showDeleteModal}
	title={m.users_delete_title()}
	message={m.users_delete_confirm_message()}
	confirmText={m.common_delete()}
	resourceName={userToDelete?.username || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		userToDelete = null;
	}}
/>

<ConfirmModal
	show={!!resetUser}
	title={m.totp_reset()}
	message={m.totp_reset_help({ name: resetUser?.username || '' })}
	confirmText={m.totp_reset()}
	onConfirm={resetTOTP}
	onCancel={() => {
		if (!resetting) resetUser = null;
	}}
/>

<ConfirmModal
	show={!!passwordResetUser}
	title={m.users_require_password_change()}
	message={m.users_password_reset_confirm({ name: passwordResetUser?.username || '' })}
	confirmText={m.users_require_password_change()}
	onConfirm={requirePasswordChange}
	onCancel={() => (passwordResetUser = null)}
/>
