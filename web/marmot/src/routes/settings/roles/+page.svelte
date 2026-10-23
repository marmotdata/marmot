<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import RoleManagement from '$components/role/RoleManagement.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { BTN_PRIMARY } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let createOpen = $state(false);
	let canManage = $derived(holdsAtRoot($myAccess, 'roles:manage'));
</script>

<svelte:head>
	<title>{m.settings_roles()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader title={m.settings_roles()} description={m.settings_roles_description()}>
	{#snippet actions()}
		{#if canManage}
			<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
				<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
				{m.roles_new_button()}
			</button>
		{/if}
	{/snippet}
</PageHeader>
<RoleManagement bind:createOpen {canManage} />
