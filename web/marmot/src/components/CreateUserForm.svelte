<!-- CreateUserForm.svelte -->
<script lang="ts">
	import { fetchApi } from '$lib/api';

	export let onUserCreated: (user: any) => void = () => {};

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

<div class="mb-6 bg-earthy-brown-100 dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-lg p-6 animate-slide-down">
	<h4 class="text-base font-medium text-gray-900 dark:text-gray-100 dark:text-gray-100 dark:text-gray-200 mb-4">Create New User</h4>
	{#if error}
		<div class="mb-4 text-red-600">{error}</div>
	{/if}
	<div class="space-y-4">
		<div>
			<label for="username" class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Username</label>
			<input
				type="text"
				id="username"
				bind:value={newUser.username}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			/>
		</div>
		<div>
			<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Name</label>
			<input
				type="text"
				id="name"
				bind:value={newUser.name}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			/>
		</div>
		<div>
			<label for="password" class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Password</label>
			<input
				type="password"
				id="password"
				bind:value={newUser.password}
				on:input={() => validatePassword(newUser.password)}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			/>
			{#if passwordError}
				<p class="mt-1 text-sm text-red-600">{passwordError}</p>
			{/if}
		</div>
		<div>
			<label for="role" class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300">Role</label>
			<select
				id="role"
				bind:value={newUser.role_names[0]}
				class="mt-1 block w-full px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 rounded-md shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm border-gray-300 dark:border-gray-600 dark:border-gray-600 dark:border-gray-600"
			>
				{#each roles as role}
					<option value={role}>{role}</option>
				{/each}
			</select>
		</div>
		<div class="flex justify-end">
			<button
				type="button"
				class="px-4 py-2 bg-earthy-terracotta-700 text-white rounded-md hover:bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600"
				on:click={createUser}
				disabled={loading}
			>
				{#if loading}
					<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white" />
				{:else}
					Create User
				{/if}
			</button>
		</div>
	</div>
</div>
