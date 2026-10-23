<script lang="ts">
	import ApiKeys from '$components/auth/ApiKeys.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_api_keys()} · {m.account_title()}</title>
</svelte:head>

<PageHeader title={m.account_api_keys()} description={m.account_api_keys_description()} />
<ApiKeys />
