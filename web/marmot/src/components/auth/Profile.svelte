<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import ThemeToggle from '$components/ui/ThemeToggle.svelte';
	import NotificationPreferencesToggle from '$components/ui/NotificationPreferencesToggle.svelte';

	interface Permission {
		name: string;
		description: string;
		action: string;
		resource_type: string;
	}

	interface Role {
		name: string;
		permissions: Permission[];
	}

	interface User {
		name: string;
		email: string;
		roles: Role[];
	}

	let loading = true;
	let error: string | null = null;
	let user: User = {
		name: '',
		email: '',
		roles: []
	};

	onMount(fetchProfile);

	async function fetchProfile() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi('/users/me');
			if (!response.ok) {
				throw new Error('Failed to load profile');
			}
			user = await response.json();
		} catch (err) {
			console.error('Profile fetch error:', err);
			error = err instanceof Error ? err.message : 'Failed to load profile';
		} finally {
			loading = false;
		}
	}
</script>

<div
	class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 divide-y divide-gray-200 dark:divide-gray-700"
>
	<!-- Basic Information -->
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">Profile Information</h3>
		{#if loading}
			<div class="mt-4">Loading...</div>
		{:else if error}
			<div class="mt-4 text-red-600">{error}</div>
		{:else}
			<dl class="mt-4 grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-500">Name</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">{user.name}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-500">Username</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">{user.username}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-500">Account Created</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
						{new Date(user.created_at).toLocaleDateString()}
					</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-500">Last Updated</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-gray-100">
						{new Date(user.updated_at).toLocaleDateString()}
					</dd>
				</div>
			</dl>
		{/if}
	</div>

	<!-- User Preferences -->
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">User Preferences</h3>
		<div class="space-y-4">
			<div>
				<h4 class="text-sm font-medium text-gray-500 dark:text-gray-500 mb-2">Theme</h4>
				<ThemeToggle />
			</div>
			<div>
				<h4 class="text-sm font-medium text-gray-500 dark:text-gray-500 mb-2">Notifications</h4>
				<NotificationPreferencesToggle />
			</div>
		</div>
	</div>

	<!-- Roles and Permissions -->
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Roles & Permissions</h3>
		<div class="space-y-6">
			<!-- Roles -->
			<div>
				<h4 class="text-sm font-medium text-gray-500 dark:text-gray-500 mb-2">Assigned Roles</h4>
				<div class="flex flex-wrap gap-2">
					{#each user.roles as role}
						<span
							class="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900 text-earthy-terracotta-700 dark:text-earthy-terracotta-100"
						>
							{role.name}
						</span>
					{/each}
				</div>
			</div>

			<!-- Permissions -->
			<div>
				<h4 class="text-sm font-medium text-gray-500 dark:text-gray-500 mb-2">Permissions</h4>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					{#each user.roles as role}
						{#each role.permissions as permission}
							<div class="bg-earthy-brown-100 dark:bg-gray-800 rounded-md p-3">
								<div class="font-medium text-gray-900 dark:text-gray-100">{permission.name}</div>
								<div class="text-sm text-gray-600 dark:text-gray-400">{permission.description}</div>
								<div class="mt-1 text-xs text-gray-500 dark:text-gray-500">
									{permission.action} on {permission.resource_type}
								</div>
							</div>
						{/each}
					{/each}
				</div>
			</div>
		</div>
	</div>

	<!-- Account Status -->
	<div class="p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Account Status</h3>
		<div class="flex items-center space-x-2">
			<span
				class={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${user.active ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}
			>
				{user.active ? 'Active' : 'Inactive'}
			</span>
		</div>
	</div>
</div>
