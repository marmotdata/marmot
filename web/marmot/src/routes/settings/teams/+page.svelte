<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import TeamManagement from '$components/team/TeamManagement.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { BTN_PRIMARY } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let createOpen = $state(false);
	let canManage = $derived(holdsAtRoot($myAccess, 'teams:manage'));
</script>

<svelte:head>
	<title>{m.settings_teams()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader title={m.settings_teams()} description={m.settings_teams_description()}>
	{#snippet actions()}
		{#if canManage}
			<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
				<IconifyIcon icon="material-symbols:group-add-outline" class="h-5 w-5" />
				{m.teams_new_button()}
			</button>
		{/if}
	{/snippet}
</PageHeader>
<TeamManagement bind:createOpen {canManage} />
