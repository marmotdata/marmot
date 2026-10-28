<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import UserManagement from '$components/user/UserManagement.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { BTN_PRIMARY } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let createOpen = $state(false);
	let canManage = $derived(holdsAtRoot($myAccess, 'users:manage'));
</script>

<svelte:head>
	<title>{m.settings_members()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader title={m.settings_members()} description={m.settings_members_description()}>
	{#snippet actions()}
		{#if canManage}
			<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
				<IconifyIcon icon="material-symbols:person-add-outline" class="h-5 w-5" />
				{m.members_add_button()}
			</button>
		{/if}
	{/snippet}
</PageHeader>
<UserManagement bind:createOpen {canManage} />
