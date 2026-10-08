<script lang="ts">
	import Sidebar from '$components/ui/Sidebar.svelte';
	import UserManagement from '$components/user/UserManagement.svelte';
	import TeamManagement from '$components/team/TeamManagement.svelte';
	import RoleManagement from '$components/role/RoleManagement.svelte';
	import SearchManagement from '$components/admin/SearchManagement.svelte';
	import SSOProvidersView from '$components/sso/SSOProvidersView.svelte';
	import ServiceAccountManagement from '$components/serviceaccount/ServiceAccountManagement.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { m } from '$lib/paraglide/messages';

	const tabs = [
		{ id: 'users', label: m.admin_tab_users() },
		{ id: 'teams', label: m.admin_tab_teams() },
		{ id: 'roles', label: m.admin_tab_roles() },
		{ id: 'service_accounts', label: m.admin_tab_service_accounts() },
		{ id: 'federation', label: m.admin_tab_authentication() },
		{ id: 'system', label: m.admin_tab_system() }
	];

	$: activeTab = $page.url.searchParams.get('tab') || tabs[0]?.id;

	onMount(() => {
		if (!$page.url.searchParams.has('tab')) {
			goto(resolve(`/admin?tab=${tabs[0]?.id}` as `/${string}`), { replaceState: true });
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
			{:else if activeTab === 'roles'}
				<div class="animate-slide-down">
					<RoleManagement />
				</div>
			{:else if activeTab === 'service_accounts'}
				<div class="animate-slide-down">
					<ServiceAccountManagement />
				</div>
			{:else if activeTab === 'federation'}
				<div class="animate-slide-down">
					<SSOProvidersView />
				</div>
			{:else if activeTab === 'system'}
				<div class="animate-slide-down">
					<SearchManagement />
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
