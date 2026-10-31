<script lang="ts">
	import { page } from '$app/state';
	import RoleEditor from '$components/role/RoleEditor.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { setCrumbs } from '$lib/mm';
	import { getRole } from '$lib/roles/api';
	import { documentFromRole, type RoleDocument } from '$lib/roles/document';
	import { settingsCrumbs } from '$lib/settings/sections';
	import { m } from '$lib/paraglide/messages';

	let canManage = $derived(holdsAtRoot($myAccess, 'roles:manage'));

	// Duplicate role opens this page with ?from=<id>, so the new role starts from that role's permissions
	let from = $derived(page.url.searchParams.get('from'));
	let initial = $state<RoleDocument | undefined>();
	let ready = $state(false);

	$effect(() => {
		const id = from;
		ready = false;
		initial = undefined;
		if (!id) {
			ready = true;
			return;
		}
		getRole(id)
			.then((role) => (initial = { ...documentFromRole(role), name: `${role.name}-copy` }))
			.catch(() => (initial = undefined))
			.finally(() => (ready = true));
	});

	setCrumbs(
		settingsCrumbs(
			{ label: m.settings_roles(), href: '/settings/roles' },
			{ label: m.roles_create_title() }
		)
	);
</script>

{#if ready}
	{#key from}
		<RoleEditor {initial} {canManage} />
	{/key}
{:else}
	<mm-skeleton loading count={4}></mm-skeleton>
{/if}
