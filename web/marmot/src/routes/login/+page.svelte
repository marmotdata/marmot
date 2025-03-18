<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Button from '../../components/Button.svelte';
	import OAuthButtons from '../../components/OAuthButtons.svelte';
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);
	let usernameInput: HTMLInputElement;
	let passwordInput: HTMLInputElement;

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
</script>

<div class="min-h-screen flex items-center justify-center bg-earthy-brown-50 dark:bg-gray-900">
	<div class="max-w-md w-full space-y-8 p-8 bg-white dark:bg-gray-800 rounded-lg shadow-md">
		<div>
			<img src="/images/marmot.svg" alt="Logo" class="mx-auto h-24 w-auto" />
			<h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-gray-100">
				Sign in to Marmot
			</h2>
		</div>

		{#if error}
			<div class="bg-red-50 dark:bg-red-900/50 text-red-600 dark:text-red-400 p-4 rounded-md">
				{error}
			</div>
		{/if}

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
	</div>
</div>
