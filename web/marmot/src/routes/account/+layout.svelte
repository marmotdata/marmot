<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { setCrumbs } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { userProfile } from '$lib/stores/user';
	import {
		apiKeys,
		subscriptions,
		connectedApps,
		loadApiKeys,
		loadSubscriptions,
		loadConnectedApps
	} from '$components/auth/accountData';

	let { children }: { children: Snippet } = $props();

	// Crumbs belong to a path, so they are set again as the tabs change the URL
	$effect(() => {
		void page.url.pathname;
		setCrumbs([{ label: m.account_title() }]);
	});

	onMount(() => {
		userProfile.load();
		loadSubscriptions();
		loadApiKeys();
		loadConnectedApps();
	});

	const tab = $derived.by(() => {
		const path = page.url.pathname;
		if (path.startsWith('/account/notifications')) return 'notifications';
		if (path.startsWith('/account/subscriptions')) return 'subscriptions';
		if (path.startsWith('/account/api-keys')) return 'keys';
		if (path.startsWith('/account/connected-apps')) return 'apps';
		return 'profile';
	});

	const name = $derived($userProfile?.name || $userProfile?.username || '');
	// Job title and teams under the name, as people introduce themselves
	const summary = $derived(
		[$userProfile?.job_title, ($userProfile?.teams ?? []).map((t) => t.name).join(', ')]
			.filter(Boolean)
			.join(', ')
	);

	async function signOut() {
		await auth.signOut();
		userProfile.clear();
		window.location.href = '/';
	}
</script>

<div class="mm-stack">
	<mm-page-header heading={name || m.account_title()} description={summary}>
		{#if name}
			<mm-avatar
				slot="leading"
				{name}
				src={$userProfile?.profile_picture ?? ''}
				size={40}
				decorative
			></mm-avatar>
		{/if}
		<mm-button slot="actions" icon="logout" onclick={signOut}>{m.account_sign_out()}</mm-button>
	</mm-page-header>

	<mm-tabs label={m.account_nav_aria()} value={tab}>
		<mm-tab value="profile" icon="account-circle" href={resolve('/account')}
			>{m.account_profile()}</mm-tab
		>
		<mm-tab value="notifications" icon="notifications" href={resolve('/account/notifications')}
			>{m.account_notifications()}</mm-tab
		>
		<mm-tab
			value="subscriptions"
			icon="bookmark"
			href={resolve('/account/subscriptions')}
			count={$subscriptions.items?.length}>{m.account_subscriptions()}</mm-tab
		>
		<mm-tab
			value="keys"
			icon="key"
			href={resolve('/account/api-keys')}
			count={$apiKeys.items?.length}>{m.account_api_keys()}</mm-tab
		>
		<mm-tab
			value="apps"
			icon="link"
			href={resolve('/account/connected-apps')}
			count={$connectedApps.items?.length}>{m.account_connected_apps()}</mm-tab
		>
	</mm-tabs>

	{@render children()}
</div>
