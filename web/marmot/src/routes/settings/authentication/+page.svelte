<script lang="ts">
	import { onMount } from 'svelte';
	import SSOProvidersView from '$components/sso/SSOProvidersView.svelte';
	import GroupMappings from '$components/sso/GroupMappings.svelte';
	import NewMemberSettings from '$components/sso/NewMemberSettings.svelte';
	import { fetchApi } from '$lib/api';
	import { listRoles } from '$lib/roles/api';
	import { m } from '$lib/paraglide/messages';
	import type { SSOProviderStatus } from '$lib/sso/types';
	import type { Role } from '$lib/roles/types';
	import type { Team } from '$lib/teams/types';

	// The mappings and the new-member form both choose from roles and teams, so they are read once here
	let providers = $state<SSOProviderStatus[]>([]);
	let roles = $state<Role[]>([]);
	let teams = $state<Team[]>([]);

	onMount(async () => {
		listRoles()
			.then((list) => (roles = list))
			.catch(() => (roles = []));
		try {
			const res = await fetchApi('/teams?limit=100');
			teams = res.ok ? ((await res.json()).teams ?? []) : [];
		} catch {
			teams = [];
		}
	});
</script>

<svelte:head>
	<title>{m.settings_authentication()} · {m.settings_title()}</title>
</svelte:head>

<mm-page-header heading={m.settings_authentication()}></mm-page-header>
<SSOProvidersView onChange={(list) => (providers = list)} />
<GroupMappings {providers} {roles} {teams} />
<NewMemberSettings {roles} {teams} />
