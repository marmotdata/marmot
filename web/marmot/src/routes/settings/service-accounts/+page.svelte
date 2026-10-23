<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import ServiceAccountManagement from '$components/serviceaccount/ServiceAccountManagement.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { BTN_PRIMARY } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	let createOpen = $state(false);
	let canManage = $derived(holdsAtRoot($myAccess, 'service_accounts:manage'));
</script>

<svelte:head>
	<title>{m.settings_service_accounts()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader
	title={m.settings_service_accounts()}
	description={m.settings_service_accounts_description()}
>
	{#snippet actions()}
		{#if canManage}
			<button type="button" class={BTN_PRIMARY} onclick={() => (createOpen = true)}>
				<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
				{m.serviceaccounts_new_button()}
			</button>
		{/if}
	{/snippet}
</PageHeader>
<ServiceAccountManagement bind:createOpen {canManage} />
