<script lang="ts">
	import TOTPSettings from '$components/auth/TOTPSettings.svelte';
	import Profile from '$components/auth/Profile.svelte';
	import PasswordSettings from '$components/auth/PasswordSettings.svelte';
	import ApiKeys from '$components/auth/ApiKeys.svelte';
	import Subscriptions from '$components/auth/Subscriptions.svelte';
	import Sidebar from '$components/ui/Sidebar.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';

	let totpAvailable = false;
	let localAccount = false;
	$: tabs = [
		{ id: 'profile', label: m.profile_tab_profile() },
		{ id: 'subscriptions', label: m.profile_tab_subscriptions() },
		{ id: 'api-keys', label: m.profile_tab_api_keys() },
		...(localAccount ? [{ id: 'password', label: m.profile_change_password() }] : []),
		...(totpAvailable ? [{ id: 'totp', label: m.totp_title() }] : [])
	];

	$: activeTab = $page.url.searchParams.get('tab') || tabs[0]?.id;

	onMount(async () => {
		if (!$page.url.searchParams.has('tab')) {
			goto(resolve(`/profile?tab=${tabs[0]?.id}`), { replaceState: true });
		}
		try {
			const user = await fetchApi('/users/me');
			if (user.ok) localAccount = !(await user.json()).identities?.length;
			const config = await fetch('/auth-providers');
			if (config.ok && (await config.json()).totp_enabled) {
				const response = await fetchApi('/users/totp');
				if (response.ok) totpAvailable = localAccount && (await response.json()).local;
			}
		} catch {
			// The profile remains usable when the optional TOTP status is unavailable.
		}
		if ($page.url.searchParams.get('tab') === 'totp' && !totpAvailable) {
			goto(resolve('/profile?tab=profile'), { replaceState: true });
		}
		if ($page.url.searchParams.get('tab') === 'password' && !localAccount) {
			goto(resolve('/profile?tab=profile'), { replaceState: true });
		}
	});
</script>

<div class="container max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
	<div class="flex flex-col lg:flex-row gap-6">
		<Sidebar {tabs} />

		<div class="flex-1">
			{#if activeTab === 'profile'}
				<Profile />
			{:else if activeTab === 'subscriptions'}
				<Subscriptions />
			{:else if activeTab === 'api-keys'}
				<ApiKeys />
			{:else if activeTab === 'password' && localAccount}
				<PasswordSettings />
			{:else if activeTab === 'totp' && totpAvailable}
				<TOTPSettings />
			{/if}
		</div>
	</div>
</div>
