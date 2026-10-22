<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';

	// Browser sign-in covers people; this points unattended agents at a service account key instead
</script>

<a
	href={resolve('/admin?tab=service_accounts')}
	class="group inline-flex items-center gap-1.5 rounded-md bg-earthy-blue-50 px-2.5 py-1 text-xs text-earthy-blue-800 transition hover:bg-earthy-blue-100 dark:bg-earthy-blue-900/30 dark:text-earthy-blue-200 dark:hover:bg-earthy-blue-900/50"
>
	<IconifyIcon icon="material-symbols:info-outline" class="h-3.5 w-3.5" />
	{m.home_connect_agent_headless()}
	<span class="font-semibold">{m.home_connect_agent_service_account()}</span>
	<IconifyIcon
		icon="material-symbols:arrow-forward"
		class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5"
	/>
</a>
