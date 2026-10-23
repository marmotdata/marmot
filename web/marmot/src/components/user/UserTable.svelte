<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { BTN_GHOST, TABLE, TBODY, TD, TH, TR } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import DeleteModal from '$components/ui/DeleteModal.svelte';
	import EditUserForm from './EditUserForm.svelte';
	import type { User } from '$lib/users/types';

	let {
		users = [],
		editingUserId = null,
		canManage = true,
		onEdit,
		onUpdate,
		onDelete
	}: {
		users?: User[];
		editingUserId?: string | null;
		canManage?: boolean;
		onEdit: (userId: string | null) => void;
		onUpdate: (user: User) => void;
		onDelete: (userId: string) => void;
	} = $props();

	let editingUser = $derived(users.find((u) => u.id === editingUserId) ?? null);
	let showDeleteModal = $state(false);
	let userToDelete = $state<User | null>(null);

	const currentUserId = auth.getCurrentUserId();

	function providerDisplayName(provider: string): string {
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

	function initials(user: User): string {
		const source = (user.name || user.username).trim();
		const parts = source.split(/\s+/);
		return (
			parts.length >= 2 ? parts[0][0] + parts[parts.length - 1][0] : source.slice(0, 2)
		).toUpperCase();
	}

	async function handleDelete() {
		if (!userToDelete) return;
		try {
			const response = await fetchApi(`/users/${userToDelete.id}`, { method: 'DELETE' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
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
</script>

<div class="overflow-x-auto">
	<table class={TABLE}>
		<thead class="border-b border-gray-100 dark:border-gray-700/60">
			<tr>
				<th class="{TH} pl-5">{m.members_col_member()}</th>
				<th class={TH}>{m.users_header_auth()}</th>
				<th class={TH}>{m.users_header_roles()}</th>
				<th class={TH}>{m.common_status()}</th>
				<th class="{TH} pr-5 text-right"><span class="sr-only">{m.common_actions()}</span></th>
			</tr>
		</thead>
		<tbody class={TBODY}>
			{#each users as user (user.id)}
				<tr class={TR}>
					<td class="{TD} pl-5">
						<span class="flex min-w-0 items-center gap-3">
							{#if user.profile_picture}
								<img
									src={user.profile_picture}
									alt=""
									class="h-8 w-8 flex-shrink-0 rounded-full object-cover"
								/>
							{:else}
								<span
									class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full border border-gray-100 bg-gray-50 text-xs font-semibold text-gray-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
									aria-hidden="true">{initials(user)}</span
								>
							{/if}
							<span class="min-w-0">
								<span class="block truncate font-medium text-gray-900 dark:text-gray-100"
									>{user.name || user.username}</span
								>
								<span class="block truncate font-mono text-xs text-gray-500 dark:text-gray-400"
									>{user.username}</span
								>
							</span>
							{#if currentUserId === user.id}
								<Pill tone="brand">{m.members_you_badge()}</Pill>
							{/if}
						</span>
					</td>
					<td class={TD}>
						<span class="flex flex-wrap gap-1">
							{#if user.identities && user.identities.length > 0}
								{#each user.identities as identity (identity.provider)}
									<Pill>
										<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
										{providerDisplayName(identity.provider)}
									</Pill>
								{/each}
							{:else}
								<Pill>
									<IconifyIcon icon="material-symbols:mail-outline" class="h-3 w-3" />
									{m.users_auth_password_badge()}
								</Pill>
							{/if}
						</span>
					</td>
					<td class={TD}>
						<span class="flex flex-wrap gap-1">
							{#each user.roles as role (role.name)}
								<Pill tone={role.name === 'admin' ? 'brand' : 'neutral'}>{role.name}</Pill>
							{:else}
								<span class="text-gray-400 dark:text-gray-500">—</span>
							{/each}
						</span>
					</td>
					<td class={TD}>
						<Pill tone={user.active ? 'success' : 'neutral'}>
							{user.active ? m.common_active() : m.common_inactive()}
						</Pill>
					</td>
					<td class="{TD} whitespace-nowrap pr-5 text-right">
						{#if canManage && currentUserId !== user.id}
							<button
								type="button"
								class={BTN_GHOST}
								onclick={() => onEdit(user.id)}
								aria-label={m.members_edit_aria({ name: user.username })}
								title={m.common_edit()}
							>
								<IconifyIcon icon="material-symbols:edit-outline" class="h-4 w-4" />
							</button>
						{/if}
						{#if canManage && currentUserId !== user.id && user.username !== 'admin'}
							<button
								type="button"
								class="{BTN_GHOST} hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
								onclick={() => {
									userToDelete = user;
									showDeleteModal = true;
								}}
								aria-label={m.members_delete_aria({ name: user.username })}
								title={m.common_delete()}
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

{#if editingUser}
	<EditUserForm
		user={editingUser}
		onCancel={() => onEdit(null)}
		onUpdate={(updatedUser) => onUpdate(updatedUser)}
	/>
{/if}

<DeleteModal
	show={showDeleteModal}
	title={m.users_delete_title()}
	message={m.members_delete_message()}
	confirmText={m.common_delete()}
	resourceName={userToDelete?.username || ''}
	requireConfirmation={true}
	onConfirm={handleDelete}
	onCancel={() => {
		showDeleteModal = false;
		userToDelete = null;
	}}
/>
