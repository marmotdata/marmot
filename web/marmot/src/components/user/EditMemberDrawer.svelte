<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import type { MmSwitchDetail } from '@marmotdata/elements/classes/switch.js';
	import '@marmotdata/elements/multiselect.js';
	import { fetchApi } from '$lib/api';
	import { listRoles } from '$lib/roles/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { providerName, roleLabel } from '$lib/users/format';
	import type { User } from '$lib/users/types';
	import type { Role } from '$lib/roles/types';

	let { user, onClose, onSaved }: { user: User; onClose: () => void; onSaved: () => void } =
		$props();

	// The drawer edits a copy taken when it opens; the page remounts it for each member
	let name = $state(untrack(() => user.name));
	let active = $state(untrack(() => user.active));
	let roleNames = $state(untrack(() => user.roles.map((r) => r.name)));
	let roles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state('');

	// The picker works in role ids while the API names roles, so names are mapped once the roles arrive
	let roleIds = $derived(
		roleNames.map((n) => roles.find((r) => r.name === n)?.id).filter((id): id is string => !!id)
	);

	onMount(async () => {
		try {
			roles = await listRoles();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.users_error_load_roles());
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!roleNames.length) {
			error = m.users_error_select_role();
			return;
		}
		saving = true;
		error = '';
		try {
			const response = await fetchApi(`/users/${user.id}`, {
				method: 'PUT',
				body: JSON.stringify({ name: name.trim(), active, role_names: roleNames })
			});
			if (!response.ok) {
				error = await handleApiError(response);
				return;
			}
			toasts.success(m.users_update_success());
			onSaved();
		} catch (err) {
			error = err instanceof Error ? err.message : m.users_error_generic();
		} finally {
			saving = false;
		}
	}

	const val = (e: Event) => (e.currentTarget as unknown as { value: string }).value;
</script>

<mm-drawer open heading={m.members_edit_member()} description={user.username} onmm-close={onClose}>
	<form
		id="edit-member"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		<mm-text-field
			label={m.common_name()}
			name="name"
			required
			value={name}
			oninput={(e: Event) => (name = val(e))}
		></mm-text-field>
		<mm-multiselect
			label={m.users_header_roles()}
			name="roles"
			required
			value={roleIds}
			error={error && !roleNames.length ? error : ''}
			onmm-change={(e: CustomEvent<{ value: string[] }>) => {
				roleNames = e.detail.value
					.map((id: string) => roles.find((r) => r.id === id)?.name)
					.filter((n): n is string => !!n);
			}}
		>
			{#each roles as role (role.id)}
				<mm-option value={role.id} label={roleLabel(role.name)} description={role.description}
				></mm-option>
			{/each}
		</mm-multiselect>
		<mm-switch
			label={m.members_can_sign_in()}
			description={m.members_active_hint()}
			checked={active}
			onmm-change={(e: CustomEvent<MmSwitchDetail>) => (active = e.detail.checked)}
		></mm-switch>
		<mm-description-list layout="stacked">
			<mm-description term={m.members_col_signs_in_with()}>
				{#if user.identities?.length}
					{user.identities
						.map((i) =>
							i.provider_email
								? `${providerName(i.provider)} (${i.provider_email})`
								: providerName(i.provider)
						)
						.join(', ')}
				{:else}
					{m.users_auth_password_badge()}
				{/if}
			</mm-description>
		</mm-description-list>
		{#if error && roleNames.length}
			<mm-callout tone="danger" heading={m.members_save_failed()}>{error}</mm-callout>
		{/if}
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button slot="footer" variant="primary" type="submit" form="edit-member" loading={saving}>
		{m.serviceaccounts_save_changes()}
	</mm-button>
</mm-drawer>
