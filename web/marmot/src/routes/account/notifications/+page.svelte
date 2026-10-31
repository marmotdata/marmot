<script lang="ts">
	import NotificationPreferences from '$components/auth/NotificationPreferences.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.account_notifications()} · {m.account_title()}</title>
</svelte:head>

<NotificationPreferences />
