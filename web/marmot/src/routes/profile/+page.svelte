<script lang="ts">
	import Profile from '$components/auth/Profile.svelte';
	import ApiKeys from '$components/auth/ApiKeys.svelte';
	import Subscriptions from '$components/auth/Subscriptions.svelte';
	import Sidebar from '$components/ui/Sidebar.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	const tabs = [
		{ id: 'profile', label: 'Profile' },
		{ id: 'subscriptions', label: 'Subscriptions' },
		{ id: 'api-keys', label: 'API Keys' }
	];

	$: activeTab = $page.url.searchParams.get('tab') || tabs[0]?.id;

	onMount(() => {
		if (!$page.url.searchParams.has('tab')) {
			goto(`?tab=${tabs[0]?.id}`, { replaceState: true });
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
			{/if}
		</div>
	</div>
</div>
