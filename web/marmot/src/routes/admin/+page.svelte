<script lang="ts">
	import Sidebar from '../../components/Sidebar.svelte';
	import UserManagement from '../../components/UserManagement.svelte';
	import TeamManagement from '../../components/TeamManagement.svelte';
	// import RoleManagement from '../../components/RoleManagement.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	const tabs = [
		{ id: 'users', label: 'Users' },
		{ id: 'teams', label: 'Teams' }
		// { id: 'roles', label: 'Roles & Permissions' }
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
			{#if activeTab === 'users'}
				<div class="animate-slide-down">
					<UserManagement />
				</div>
			{:else if activeTab === 'teams'}
				<div class="animate-slide-down">
					<TeamManagement />
				</div>
				<!-- {:else if activeTab === 'roles'} -->
				<!-- 	<div class="animate-slide-down"> -->
				<!-- 		<RoleManagement /> -->
				<!-- 	</div> -->
			{/if}
		</div>
	</div>
</div>

<style>
	.animate-slide-down {
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
