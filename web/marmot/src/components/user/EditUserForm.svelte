<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { listRoles } from '$lib/roles/api';
	import IconifyIcon from '@iconify/svelte';
	import { BTN_PRIMARY, BTN_SECONDARY, INPUT, LABEL } from '$lib/ui/classes';
	import Pill from '$components/ui/Pill.svelte';
	import Blade from '$components/ui/Blade.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import type { User, Role } from '$lib/users/types';
	import type { Role as FullRole } from '$lib/roles/types';

	let {
		user,
		onCancel,
		onUpdate
	}: {
		user: User;
		onCancel: () => void;
		onUpdate: (updatedUser: User) => void;
	} = $props();

	let loading = $state(false);
	let availableRoles: FullRole[] = $state([]);
	// The form edits a copy taken when it opens; the row remounts it for each user
	let editedUser = $state(untrack(() => ({ ...user })));

	onMount(async () => {
		try {
			availableRoles = await listRoles();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_load_roles());
		}
	});

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

	// The picker works in role ids; the user record and the API name roles
	let selectedRoleIds = $derived(
		editedUser.roles
			.map((r: Role) => availableRoles.find((a) => a.name === r.name)?.id)
			.filter((id): id is string => !!id)
	);

	function setRoles(ids: string[]) {
		editedUser.roles = ids
			.map((id) => availableRoles.find((r) => r.id === id))
			.filter((r): r is FullRole => !!r)
			.map((r) => ({ id: r.id, name: r.name, description: r.description }));
	}

	let dirty = $derived(
		editedUser.name !== user.name ||
			editedUser.active !== user.active ||
			editedUser.roles
				.map((r: Role) => r.name)
				.sort()
				.join() !==
				user.roles
					.map((r: Role) => r.name)
					.sort()
					.join()
	);

	async function updateUser() {
		try {
			loading = true;
			const response = await fetchApi(`/users/${user.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: editedUser.name,
					active: editedUser.active,
					role_names: editedUser.roles.map((r: Role) => r.name)
				})
			});

			if (!response.ok) {
				const errorMsg = await handleApiError(response);
				toasts.error(errorMsg);
				return;
			}

			const updatedUser = await response.json();
			toasts.success(m.users_update_success());
			onUpdate(updatedUser);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_generic());
		} finally {
			loading = false;
		}
	}
</script>

<Blade
	title={user.name || user.username}
	subtitle={user.username}
	onClose={onCancel}
	width="xl"
	{dirty}
>
	<div class="space-y-6">
		<div>
			<label for="edit-user-name-{user.id}" class="{LABEL} mb-1.5 block">{m.common_name()}</label>
			<input id="edit-user-name-{user.id}" type="text" bind:value={editedUser.name} class={INPUT} />
		</div>

		<div class="flex items-start justify-between gap-4">
			<div>
				<p
					id="edit-user-status-{user.id}"
					class="text-sm font-medium text-gray-900 dark:text-gray-100"
				>
					{m.users_account_status_label()}
				</p>
				<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{m.members_active_hint()}</p>
			</div>
			<button
				type="button"
				role="switch"
				aria-checked={editedUser.active}
				aria-labelledby="edit-user-status-{user.id}"
				onclick={() => (editedUser.active = !editedUser.active)}
				class="relative inline-flex h-6 w-11 flex-shrink-0 rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-earthy-terracotta-500/40 {editedUser.active
					? 'bg-earthy-terracotta-600'
					: 'bg-gray-200 dark:bg-gray-700'}"
			>
				<span
					class="inline-block h-5 w-5 translate-y-0.5 rounded-full bg-white shadow transition-transform {editedUser.active
						? 'translate-x-5'
						: 'translate-x-0.5'}"
				></span>
			</button>
		</div>

		<div>
			<p class="{LABEL} mb-1.5">{m.users_auth_method_heading()}</p>
			<div class="flex flex-wrap gap-1.5">
				{#if user.identities && user.identities.length > 0}
					{#each user.identities as identity (identity.provider + (identity.provider_email ?? ''))}
						<Pill>
							<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
							{providerDisplayName(identity.provider)}
							{#if identity.provider_email}
								<span class="text-gray-500 dark:text-gray-400">({identity.provider_email})</span>
							{/if}
						</Pill>
					{/each}
				{:else}
					<Pill>
						<IconifyIcon icon="material-symbols:mail-outline" class="h-3 w-3" />
						{m.users_auth_password_method()}
					</Pill>
				{/if}
			</div>
		</div>

		<div>
			<p class="{LABEL} mb-1.5">{m.users_header_roles()}</p>
			{#if availableRoles.length > 0}
				<RoleSelector roles={availableRoles} selectedIds={selectedRoleIds} onChange={setRoles} />
			{/if}
		</div>
	</div>

	{#snippet footer(close)}
		<div class="flex justify-end gap-2">
			<button type="button" class={BTN_SECONDARY} onclick={close}>{m.common_cancel()}</button>
			<button type="button" class={BTN_PRIMARY} onclick={updateUser} disabled={loading || !dirty}>
				{loading ? m.members_saving_label() : m.users_save_changes()}
			</button>
		</div>
	{/snippet}
</Blade>
