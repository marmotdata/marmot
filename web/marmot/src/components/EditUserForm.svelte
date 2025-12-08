<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { Lock, Mail, User as UserIcon, Shield } from 'lucide-svelte';

	let { user, onCancel, onUpdate } = $props<{
		user: any;
		onCancel: () => void;
		onUpdate: (updatedUser: any) => void;
	}>();

	let loading = false;
	let error: string | null = null;
	let editedUser = { ...user };

	async function updateUser() {
		try {
			loading = true;
			const response = await fetchApi(`/users/${user.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: editedUser.name,
					active: editedUser.active,
					role_names: editedUser.roles.map((r: any) => r.name)
				})
			});

			if (!response.ok) {
				throw new Error('Failed to update user');
			}

			const updatedUser = await response.json();
			onUpdate(updatedUser);
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	function getProviderDisplay(provider: string): string {
		const providerMap: Record<string, string> = {
			google: 'Google',
			github: 'GitHub',
			gitlab: 'GitLab',
			okta: 'Okta',
			slack: 'Slack',
			auth0: 'Auth0'
		};
		return providerMap[provider] || provider.charAt(0).toUpperCase() + provider.slice(1);
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 m-4"
>
	<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-6 flex items-center">
		<UserIcon class="h-5 w-5 mr-2 text-gray-500 dark:text-gray-400" />
		Edit User
	</h3>

	{#if error}
		<div
			class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md text-red-700 dark:text-red-400 text-sm"
		>
			{error}
		</div>
	{/if}

	<div class="space-y-6">
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Username
				</label>
				<div
					class="px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-md text-sm text-gray-500 dark:text-gray-400"
				>
					{user.username}
				</div>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Name
				</label>
				<input
					type="text"
					bind:value={editedUser.name}
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3 flex items-center">
				<Lock class="h-4 w-4 mr-2 text-gray-500 dark:text-gray-400" />
				Authentication Method
			</h4>
			<div class="flex flex-wrap gap-2">
				{#if user.identities && user.identities.length > 0}
					{#each user.identities as identity}
						<div
							class="inline-flex items-center px-3 py-2 rounded-md text-sm font-medium bg-blue-50 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200 border border-blue-200 dark:border-blue-800"
						>
							<Lock class="h-4 w-4 mr-2" />
							{getProviderDisplay(identity.provider)}
							{#if identity.provider_email}
								<span class="ml-2 text-xs text-blue-600 dark:text-blue-400">
									({identity.provider_email})
								</span>
							{/if}
						</div>
					{/each}
				{:else}
					<div
						class="inline-flex items-center px-3 py-2 rounded-md text-sm font-medium bg-gray-50 text-gray-800 dark:bg-gray-900/50 dark:text-gray-200 border border-gray-200 dark:border-gray-700"
					>
						<Mail class="h-4 w-4 mr-2" />
						Password Authentication
					</div>
				{/if}
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3 flex items-center">
				<Shield class="h-4 w-4 mr-2 text-gray-500 dark:text-gray-400" />
				Roles
			</h4>
			<div class="space-y-2">
				{#each user.roles as role}
					<label
						class="flex items-center p-2 rounded-md hover:bg-gray-50 dark:hover:bg-gray-800/50 cursor-pointer"
					>
						<input
							type="checkbox"
							checked={editedUser.roles.some((r: any) => r.name === role.name)}
							on:change={(e) => {
								if (e.currentTarget.checked) {
									editedUser.roles = [...editedUser.roles, role];
								} else {
									editedUser.roles = editedUser.roles.filter((r: any) => r.name !== role.name);
								}
							}}
							class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500"
						/>
						<span class="ml-3 text-sm text-gray-700 dark:text-gray-300">
							{role.name}
							{#if role.description}
								<span class="text-xs text-gray-500 dark:text-gray-400 ml-1">
									— {role.description}
								</span>
							{/if}
						</span>
					</label>
				{/each}
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
				Account Status
			</label>
			<select
				bind:value={editedUser.active}
				class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
			>
				<option value={true}>Active</option>
				<option value={false}>Inactive</option>
			</select>
		</div>
	</div>

	<div class="flex justify-end space-x-3 mt-6 pt-6 border-t border-gray-200 dark:border-gray-700">
		<button
			type="button"
			class="px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 text-sm font-medium"
			on:click={onCancel}
		>
			Cancel
		</button>
		<button
			type="button"
			class="px-4 py-2 bg-earthy-terracotta-600 text-white rounded-md hover:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
			on:click={updateUser}
			disabled={loading}
		>
			{#if loading}
				<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
			{/if}
			Save Changes
		</button>
	</div>
</div>
