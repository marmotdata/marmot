<script lang="ts">
	import Profile from '../../components/Profile.svelte';
	import ApiKeys from '../../components/ApiKeys.svelte';
	import Sidebar from '../../components/Sidebar.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	const tabs = [
		{ id: 'profile', label: 'Profile' },
		{ id: 'api-keys', label: 'API Keys' }
	];

	// Get the current tab from the URL, or default to the first tab's ID
	$: activeTab = $page.url.searchParams.get('tab') || tabs[0]?.id;

	onMount(() => {
		// If there's no 'tab' query parameter, set it to the default tab
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
			{:else if activeTab === 'api-keys'}
				<ApiKeys />
			{/if}
		</div>
	</div>
</div>
