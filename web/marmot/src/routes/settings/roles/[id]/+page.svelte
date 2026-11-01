<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import RoleEditor from '$components/role/RoleEditor.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { setCrumbs } from '$lib/mm';
	import { getRole } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import { settingsCrumbs } from '$lib/settings/sections';
	import { m } from '$lib/paraglide/messages';

	let canManage = $derived(holdsAtRoot($myAccess, 'roles:manage'));
	let role = $state<Role | null>(null);
	let error = $state('');

	$effect(() => {
		const id = page.params.id ?? '';
		role = null;
		error = '';
		getRole(id)
			.then((loaded) => (role = loaded))
			.catch((err) => (error = err instanceof Error ? err.message : m.roles_error_generic()));
	});

	$effect(() => {
		setCrumbs(
			settingsCrumbs(
				{ label: m.settings_roles(), href: '/settings/roles' },
				{ label: role?.name ?? m.settings_roles() }
			)
		);
	});
</script>

{#if error}
	<mm-empty-state heading={m.roles_not_found()} description={m.roles_not_found_description()}>
		<mm-button slot="action" href={resolve('/settings/roles')}>{m.roles_back_to_roles()}</mm-button>
	</mm-empty-state>
{:else if !role}
	<mm-skeleton loading count={4}></mm-skeleton>
{:else}
	{#key role.id}
		<RoleEditor {role} {canManage} onSaved={(saved) => (role = saved)} />
	{/key}
{/if}
