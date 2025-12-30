<script lang="ts">
	import Sidebar from '$components/ui/Sidebar.svelte';
	import UserManagement from '$components/user/UserManagement.svelte';
	import TeamManagement from '$components/team/TeamManagement.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	const tabs = [
		{ id: 'users', label: 'Users' },
		{ id: 'teams', label: 'Teams' }
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
			{#if activeTab === 'users'}
				<div class="animate-slide-down">
					<UserManagement />
				</div>
			{:else if activeTab === 'teams'}
				<div class="animate-slide-down">
					<TeamManagement />
				</div>
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
