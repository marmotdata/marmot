<script lang="ts">
	import Profile from '$components/auth/Profile.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_profile()} · {m.account_title()}</title>
</svelte:head>

<PageHeader title={m.account_profile()} description={m.account_profile_description()} />
<Profile />
