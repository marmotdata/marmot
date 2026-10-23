<script lang="ts">
	import Subscriptions from '$components/auth/Subscriptions.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_subscriptions()} · {m.account_title()}</title>
</svelte:head>

<PageHeader title={m.account_subscriptions()} description={m.account_subscriptions_description()} />
<Subscriptions />
