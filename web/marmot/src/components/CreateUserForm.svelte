<!-- CreateUserForm.svelte -->
<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { User as UserIcon, Lock, Shield } from 'lucide-svelte';

	export let onUserCreated: (user: any) => void = () => {};
	export let onCancel: () => void = () => {};

	let loading = false;
	let error: string | null = null;
	let passwordError = '';

	const roles = ['user', 'admin'];

	let newUser = {
		username: '',
		name: '',
		password: '',
		role_names: ['user']
	};

	function validatePassword(password: string) {
		if (password.length < 8) {
			passwordError = 'Password must be at least 8 characters';
			return false;
		}
		passwordError = '';
		return true;
	}

	async function createUser() {
		if (!validatePassword(newUser.password)) {
			return;
		}

		try {
			loading = true;
			const response = await fetchApi('/users', {
				method: 'POST',
				body: JSON.stringify(newUser)
			});

			if (!response.ok) {
				const data = await response.json();
				throw new Error(data.error || 'Failed to create user');
			}

			const createdUser = await response.json();
			onUserCreated(createdUser);
			newUser = { username: '', name: '', password: '', role_names: ['user'] };
			error = null;
			passwordError = '';
		} catch (err: any) {
			error = err.message;
		} finally {
			loading = false;
		}
	}
</script>

<div
	class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 m-4 animate-slide-down"
>
	<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-6 flex items-center">
		<UserIcon class="h-5 w-5 mr-2 text-gray-500 dark:text-gray-400" />
		Create New User
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
				<label
					for="username"
					class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
				>
					Username
				</label>
				<input
					type="text"
					id="username"
					bind:value={newUser.username}
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>

			<div>
				<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Name
				</label>
				<input
					type="text"
					id="name"
					bind:value={newUser.name}
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3 flex items-center">
				<Lock class="h-4 w-4 mr-2 text-gray-500 dark:text-gray-400" />
				Password
			</h4>
			<div>
				<input
					type="password"
					id="password"
					bind:value={newUser.password}
					on:input={() => validatePassword(newUser.password)}
					placeholder="Minimum 8 characters"
					class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
				/>
				{#if passwordError}
					<p class="mt-2 text-sm text-red-600 dark:text-red-400">{passwordError}</p>
				{/if}
			</div>
		</div>

		<div class="border-t border-gray-200 dark:border-gray-700 pt-6">
			<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3 flex items-center">
				<Shield class="h-4 w-4 mr-2 text-gray-500 dark:text-gray-400" />
				Role
			</h4>
			<select
				id="role"
				bind:value={newUser.role_names[0]}
				class="w-full px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent"
			>
				{#each roles as role}
					<option value={role}>{role.charAt(0).toUpperCase() + role.slice(1)}</option>
				{/each}
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
			on:click={createUser}
			disabled={loading}
		>
			{#if loading}
				<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
			{/if}
			Create User
		</button>
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
