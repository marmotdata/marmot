<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Button from '../../components/Button.svelte';
	import OAuthButtons from '../../components/OAuthButtons.svelte';
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth';
	import Icon from '@iconify/svelte';

	let username = $state('');
	let password = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);
	let showPasswordChangeForm = $state(false);
	let loginData: any = $state(null);
	let usernameInput = $state<HTMLInputElement>();
	let passwordInput = $state<HTMLInputElement>();
	let newPasswordInput = $state<HTMLInputElement>();
	let confirmPasswordInput = $state<HTMLInputElement>();
	let redirectUri = $state('');

	onMount(() => {
		usernameInput?.focus();
		redirectUri = $page.url.searchParams.get('redirect_uri') || '';
	});

	function handleUsernameKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && username) {
			passwordInput?.focus();
		}
	}

	function handlePasswordKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && password) {
			handleSubmit();
		}
	}

	function handleNewPasswordKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && newPassword) {
			confirmPasswordInput?.focus();
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
			let url = '/api/v1/users/login';
			if (redirectUri) {
				url += `?redirect_uri=${encodeURIComponent(redirectUri)}`;
			}

			const response = await fetch(url, {
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
				if (data.redirect_uri) {
					window.location.href = `${data.redirect_uri}?token=${data.access_token}`;
				} else {
					// Normal web login
					auth.setToken(data.access_token);
					const redirectTo = $page.url.searchParams.get('redirect') || '/';
					goto(redirectTo);
				}
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

<div class="min-h-screen flex items-center justify-center bg-earthy-brown-50 dark:bg-gray-900 px-4">
	<div class="max-w-md w-full space-y-6">
		<!-- Logo and Title -->
		<div class="text-center">
			<div class="flex justify-center mb-6">
				<img src="/images/marmot.svg" alt="Marmot" class="h-20 w-20" />
			</div>
			<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100">
				{showPasswordChangeForm ? 'Change Password' : 'Sign In'}
			</h1>
			{#if showPasswordChangeForm}
				<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
					You must change your password before continuing
				</p>
			{/if}
		</div>

		<!-- Main Card -->
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-lg p-8"
		>
			{#if error}
				<div
					class="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400 px-4 py-3 rounded-lg flex items-start gap-3"
				>
					<Icon icon="material-symbols:error-outline" class="w-5 h-5 flex-shrink-0 mt-0.5" />
					<p class="text-sm">{error}</p>
				</div>
			{/if}

			{#if !showPasswordChangeForm}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleSubmit();
					}}
					class="space-y-5"
				>
					<div class="space-y-4">
						<div>
							<label
								for="username"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Username
							</label>
							<input
								bind:this={usernameInput}
								id="username"
								bind:value={username}
								type="text"
								required
								onkeydown={handleUsernameKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder="Enter your username"
							/>
						</div>
						<div>
							<label
								for="password"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Password
							</label>
							<input
								bind:this={passwordInput}
								id="password"
								bind:value={password}
								type="password"
								required
								onkeydown={handlePasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder="Enter your password"
							/>
						</div>
					</div>

					<Button
						class="w-full justify-center"
						type="submit"
						{loading}
						text="Sign in"
						variant="filled"
					/>
				</form>

				<OAuthButtons {redirectUri} />
			{:else}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						handlePasswordChange();
					}}
					class="space-y-5"
				>
					<div class="space-y-4">
						<div>
							<label
								for="new-password"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								New Password
							</label>
							<input
								bind:this={newPasswordInput}
								id="new-password"
								bind:value={newPassword}
								type="password"
								required
								onkeydown={handleNewPasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder="Enter new password (min 8 characters)"
							/>
						</div>
						<div>
							<label
								for="confirm-password"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Confirm Password
							</label>
							<input
								bind:this={confirmPasswordInput}
								id="confirm-password"
								bind:value={confirmPassword}
								type="password"
								required
								onkeydown={handleConfirmPasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder="Confirm new password"
							/>
						</div>
					</div>

					<div class="space-y-3">
						<Button
							class="w-full justify-center"
							type="submit"
							{loading}
							text="Change Password"
							variant="filled"
						/>
						<Button
							class="w-full justify-center"
							text="Back to Login"
							variant="clear"
							icon="material-symbols:arrow-back"
							click={goBackToLogin}
						/>
					</div>
				</form>
			{/if}
		</div>
	</div>
</div>
