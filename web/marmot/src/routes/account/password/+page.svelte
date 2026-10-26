<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PasswordSettings from '$components/auth/PasswordSettings.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { accountCapabilities } from '$lib/account/capabilities';
	import { m } from '$lib/paraglide/messages';

	let allowed = $state(false);

	onMount(async () => {
		allowed = (await accountCapabilities()).localAccount;
		if (!allowed) goto(resolve('/account'), { replaceState: true });
	});
</script>

<svelte:head>
	<title>{m.profile_change_password()} · {m.account_title()}</title>
</svelte:head>

<PageHeader title={m.profile_change_password()} />
{#if allowed}
	<PasswordSettings />
{/if}
