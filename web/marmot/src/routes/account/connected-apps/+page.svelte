<script lang="ts">
	import ConnectedApps from '$components/auth/ConnectedApps.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_connected_apps()} · {m.account_title()}</title>
</svelte:head>

<ConnectedApps />
