<script lang="ts">
	import Subscriptions from '$components/auth/Subscriptions.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_subscriptions()} · {m.account_title()}</title>
</svelte:head>

<Subscriptions />
