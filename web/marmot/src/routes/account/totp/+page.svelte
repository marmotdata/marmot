<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import TOTPSettings from '$components/auth/TOTPSettings.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { accountCapabilities } from '$lib/account/capabilities';
	import { m } from '$lib/paraglide/messages';

	let allowed = $state(false);

	onMount(async () => {
		allowed = (await accountCapabilities()).totpAvailable;
		if (!allowed) goto(resolve('/account'), { replaceState: true });
	});
</script>

<svelte:head>
	<title>{m.totp_title()} · {m.account_title()}</title>
</svelte:head>

<PageHeader title={m.totp_title()} />
{#if allowed}
	<TOTPSettings />
{/if}
