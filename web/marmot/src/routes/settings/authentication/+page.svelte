<script lang="ts">
	import SSOProvidersView from '$components/sso/SSOProvidersView.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.settings_authentication()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader
	title={m.settings_authentication()}
	description={m.settings_authentication_description()}
/>
<SSOProvidersView />
