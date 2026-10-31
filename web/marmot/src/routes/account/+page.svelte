<script lang="ts">
	import Profile from '$components/auth/Profile.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_profile()} · {m.account_title()}</title>
</svelte:head>

<Profile />
