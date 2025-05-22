<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Button from '../../components/Button.svelte';
	import OAuthButtons from '../../components/OAuthButtons.svelte';
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth';

	let username = $state('');
	let password = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);
	let showPasswordChangeForm = $state(false);
	let loginData: any = $state(null);
	let usernameInput: HTMLInputElement;
	let passwordInput: HTMLInputElement;
	let newPasswordInput: HTMLInputElement;
	let confirmPasswordInput: HTMLInputElement;

	onMount(() => {
		usernameInput.focus();
	});

	function handleUsernameKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && username) {
			passwordInput.focus();
		}
	}

	function handlePasswordKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && password) {
			handleSubmit();
		}
	}

	function handleNewPasswordKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && newPassword) {
			confirmPasswordInput.focus();
		}
	}

	function handleConfirmPasswordKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && confirmPassword) {
			handlePasswordChange();
		}
	}

	async function handleSubmit() {
		error = '';
		loading = true;

		try {
			const response = await fetch('/api/v1/users/login', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({ username, password })
			});

			if (!response.ok) {
				throw new Error('Invalid credentials');
			}

			const data = await response.json();
			loginData = data;

			if (data.requires_password_change) {
				showPasswordChangeForm = true;
				setTimeout(() => newPasswordInput?.focus(), 100);
				return;
			}

			if (data.access_token) {
				auth.setToken(data.access_token);
				const redirectTo = $page.url.searchParams.get('redirect') || '/';
				goto(redirectTo);
			} else {
				throw new Error('No token received from server');
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			loading = false;
		}
	}

	async function handlePasswordChange() {
		error = '';

		if (newPassword !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}

		if (newPassword.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}

		loading = true;

		try {
			const response = await fetch('/api/v1/users/update-password', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${loginData.access_token}`
				},
				body: JSON.stringify({ new_password: newPassword })
			});

			if (!response.ok) {
				throw new Error('Failed to change password');
			}

			const passwordData = await response.json();

			if (passwordData.access_token) {
				auth.setToken(passwordData.access_token);
				const redirectTo = $page.url.searchParams.get('redirect') || '/';
				goto(redirectTo);
			} else {
				throw new Error('No token received from server');
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Password change failed';
		} finally {
			loading = false;
		}
	}

	function goBackToLogin() {
		showPasswordChangeForm = false;
		newPassword = '';
		confirmPassword = '';
		error = '';
		setTimeout(() => usernameInput?.focus(), 100);
	}
</script>

<div class="min-h-screen flex items-center justify-center bg-earthy-brown-50 dark:bg-gray-900">
	<div class="max-w-md w-full space-y-8 p-8 bg-white dark:bg-gray-800 rounded-lg shadow-md">
		<div>
			<img src="/images/marmot.svg" alt="Logo" class="mx-auto h-24 w-auto" />
			<h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-gray-100">
				{showPasswordChangeForm ? 'Change Password' : 'Sign in to Marmot'}
			</h2>
			{#if showPasswordChangeForm}
				<p class="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
					You must change your password before continuing
				</p>
			{/if}
		</div>

		{#if error}
			<div class="bg-red-50 dark:bg-red-900/50 text-red-600 dark:text-red-400 p-4 rounded-md">
				{error}
			</div>
		{/if}

		{#if !showPasswordChangeForm}
			<form on:submit|preventDefault={handleSubmit} class="mt-8 space-y-6">
				<div class="rounded-md shadow-sm space-y-4">
					<div>
						<label for="username" class="sr-only">Username</label>
						<input
							bind:this={usernameInput}
							id="username"
							bind:value={username}
							type="text"
							required
							on:keydown={handleUsernameKeydown}
							class="appearance-none rounded-lg relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700"
							placeholder="Username"
						/>
					</div>
					<div>
						<label for="password" class="sr-only">Password</label>
						<input
							bind:this={passwordInput}
							id="password"
							bind:value={password}
							type="password"
							required
							on:keydown={handlePasswordKeydown}
							class="appearance-none rounded-lg relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700"
							placeholder="Password"
						/>
					</div>
				</div>

				<div>
					<Button class="w-full" type="submit" {loading} text="Sign in" variant="filled" />
				</div>
			</form>

			<OAuthButtons />
		{:else}
			<form on:submit|preventDefault={handlePasswordChange} class="mt-8 space-y-6">
				<div class="rounded-md shadow-sm space-y-4">
					<div>
						<label for="new-password" class="sr-only">New Password</label>
						<input
							bind:this={newPasswordInput}
							id="new-password"
							bind:value={newPassword}
							type="password"
							required
							on:keydown={handleNewPasswordKeydown}
							class="appearance-none rounded-lg relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700"
							placeholder="New Password"
						/>
					</div>
					<div>
						<label for="confirm-password" class="sr-only">Confirm Password</label>
						<input
							bind:this={confirmPasswordInput}
							id="confirm-password"
							bind:value={confirmPassword}
							type="password"
							required
							on:keydown={handleConfirmPasswordKeydown}
							class="appearance-none rounded-lg relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700"
							placeholder="Confirm Password"
						/>
					</div>
				</div>

				<div class="space-y-3">
					<Button class="w-full" type="submit" {loading} text="Change Password" variant="filled" />
					<button
						type="button"
						on:click={goBackToLogin}
						class="w-full px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
					>
						Back to Login
					</button>
				</div>
			</form>
		{/if}
	</div>
</div>
