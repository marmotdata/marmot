<script lang="ts">
	import ApiKeys from '#components/auth/ApiKeys.svelte';
	import { m } from '#lib/paraglide/messages.js';
</script>

<svelte:head>
	<title>{m.account_api_keys()} · {m.account_title()}</title>
</svelte:head>

<ApiKeys />
