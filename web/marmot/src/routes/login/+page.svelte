<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/stores';
	import Button from '$components/ui/Button.svelte';
	import OAuthButtons from '$components/auth/OAuthButtons.svelte';
	import TOTPChallenge from '$components/auth/TOTPChallenge.svelte';
	import TOTPSettings from '$components/auth/TOTPSettings.svelte';
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth';
	import { loginRedirect } from '$lib/auth/loginRedirect';
	import Icon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';

	let username = $state('');
	let password = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);
	let redirecting = $state(false);
	let showPasswordChangeForm = $state(false);
	interface LoginResponse {
		access_token?: string;
		requires_password_change?: boolean;
		requires_totp?: boolean;
		requires_totp_enrollment?: boolean;
		mfa_token?: string;
	}
	let loginData: LoginResponse | null = $state(null);
	let usernameInput = $state<HTMLInputElement>();
	let passwordInput = $state<HTMLInputElement>();
	let newPasswordInput = $state<HTMLInputElement>();
	let confirmPasswordInput = $state<HTMLInputElement>();
	let pendingConsent = $state<{ redirect_uri: string; client_id: string; scopes: string[] } | null>(
		null
	);

	onMount(async () => {
		if ($page.url.searchParams.has('oauth_pending')) {
			await loadPendingConsent();
		}
		if (!pendingConsent) {
			usernameInput?.focus();
		}
	});

	async function loadPendingConsent() {
		const token = auth.getToken();
		if (!token) {
			return;
		}
		// Verify the stored token still works on the server — a stale token
		// (expired, signing key rotated, server restarted) would otherwise let us
		// show the consent screen, then fail at /oauth/authorize/complete with 401.
		try {
			const meRes = await fetch('/api/v1/users/me', {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (!meRes.ok) {
				auth.clearToken();
				return;
			}
		} catch {
			return;
		}
		try {
			const response = await fetch('/oauth/authorize/pending', { credentials: 'same-origin' });
			if (response.ok) {
				pendingConsent = await response.json();
			}
		} catch {
			/* pending consent fetch failed; remain on login screen */
		}
	}

	async function handleConsentAllow() {
		error = '';
		loading = true;
		try {
			const token = auth.getToken();
			const response = await fetch('/oauth/authorize/complete', {
				method: 'POST',
				headers: { Authorization: `Bearer ${token}` },
				credentials: 'same-origin'
			});
			if (!response.ok) {
				throw new Error(m.login_error_complete_authorization());
			}
			const data = await response.json();
			if (data.oauth_redirect) {
				window.location.href = data.oauth_redirect;
				return;
			}
			throw new Error(m.login_error_no_redirect());
		} catch (err) {
			error = err instanceof Error ? err.message : m.login_error_authorization_failed();
			loading = false;
		}
	}

	async function handleConsentCancel() {
		try {
			await fetch('/oauth/authorize/cancel', {
				method: 'POST',
				credentials: 'same-origin'
			});
		} catch {
			/* best-effort cancel */
		}
		pendingConsent = null;
		goto(resolve('/'));
	}

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
		if (loading) return;
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
				throw new Error(m.login_error_invalid_credentials());
			}

			const data = await response.json();
			await finishLogin(data);
		} catch (err) {
			error = err instanceof Error ? err.message : m.login_error_login_failed();
		} finally {
			loading = false;
		}
	}

	async function finishLogin(data: LoginResponse) {
		if (data.requires_totp) {
			loginData = data;
			password = '';
			newPassword = '';
			confirmPassword = '';
			showPasswordChangeForm = false;
			return;
		}
		if (data.requires_totp_enrollment) {
			loginData = data;
			password = '';
			showPasswordChangeForm = false;
			return;
		}

		if (data.requires_password_change) {
			loginData = data;
			showPasswordChangeForm = true;
			setTimeout(() => newPasswordInput?.focus(), 100);
			return;
		}

		if (data.access_token) {
			auth.setToken(data.access_token);
			if ($page.url.searchParams.has('oauth_pending')) {
				await loadPendingConsent();
				if (pendingConsent) {
					return;
				}
			}
			const redirectTo = loginRedirect(
				$page.url.searchParams.get('redirect'),
				window.location.origin
			);
			// Full navigation applies the account language preference from the new token.
			redirecting = true;
			window.location.assign(resolve(redirectTo));
		} else {
			throw new Error(m.login_error_no_token());
		}
	}

	async function handlePasswordChange() {
		if (loading) return;
		error = '';

		if (newPassword !== confirmPassword) {
			error = m.login_error_password_mismatch();
			return;
		}

		if (newPassword.length < 8) {
			error = m.login_error_password_too_short();
			return;
		}

		loading = true;

		try {
			const response = await fetch(
				loginData?.mfa_token ? '/api/v1/users/login/password' : '/api/v1/users/update-password',
				{
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${loginData?.access_token ?? ''}`
					},
					body: JSON.stringify({ new_password: newPassword, mfa_token: loginData?.mfa_token })
				}
			);

			if (!response.ok) {
				throw new Error(m.login_error_update_password());
			}

			const passwordData = await response.json();

			await finishLogin(passwordData);
		} catch (err) {
			error = err instanceof Error ? err.message : m.login_error_password_change_failed();
		} finally {
			loading = false;
		}
	}

	async function verifyTOTP(code: string) {
		if (loading) return;
		loading = true;
		error = '';
		try {
			const response = await fetch('/api/v1/users/login/totp', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ mfa_token: loginData?.mfa_token, code })
			});
			if (!response.ok)
				throw new Error(response.status === 429 ? m.totp_rate_limited() : m.totp_invalid_code());
			await finishLogin(await response.json());
		} catch (err) {
			error = err instanceof Error ? err.message : m.totp_error();
		} finally {
			loading = false;
		}
	}

	function goBackToLogin() {
		showPasswordChangeForm = false;
		loginData = null;
		password = '';
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
				{redirecting
					? m.login_finishing_heading()
					: pendingConsent
						? m.login_authorize_heading()
						: loginData?.requires_totp
							? m.totp_title()
							: loginData?.requires_totp_enrollment
								? m.totp_enrollment_title()
								: showPasswordChangeForm
									? m.login_change_password_heading()
									: m.login_signin_heading()}
			</h1>
			{#if showPasswordChangeForm && !pendingConsent}
				<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
					{m.login_change_password_hint()}
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

			{#if redirecting}
				<div role="status" class="flex flex-col items-center gap-4 py-8 text-center">
					<div
						class="h-9 w-9 animate-spin rounded-full border-2 border-earthy-terracotta-200 border-t-earthy-terracotta-700"
						aria-hidden="true"
					/>
					<p class="text-sm text-gray-600 dark:text-gray-300">{m.login_finishing_help()}</p>
				</div>
			{:else if pendingConsent}
				<div class="space-y-5">
					<p class="text-sm text-gray-700 dark:text-gray-300">
						{m.login_consent_intro()}
					</p>
					<div
						class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-4 py-3 font-mono text-sm break-all text-gray-900 dark:text-gray-100"
					>
						{pendingConsent.redirect_uri}
					</div>
					<p class="text-xs text-gray-500 dark:text-gray-400">
						{m.login_consent_warning({ command: 'marmot login' })}
					</p>
					<div class="space-y-3">
						<Button
							class="w-full justify-center"
							{loading}
							text={m.login_authorize_button()}
							variant="filled"
							click={handleConsentAllow}
						/>
						<Button
							class="w-full justify-center"
							text={m.common_cancel()}
							variant="clear"
							click={handleConsentCancel}
						/>
					</div>
				</div>
			{:else if loginData?.requires_totp}
				<div class="space-y-5">
					<p class="text-sm text-gray-600 dark:text-gray-300">{m.totp_login_help()}</p>
					<TOTPChallenge verify={verifyTOTP} {loading} />
					<Button text={m.common_cancel()} variant="clear" click={goBackToLogin} />
				</div>
			{:else if loginData?.requires_totp_enrollment}
				<div class="space-y-5">
					<p class="text-sm text-gray-600 dark:text-gray-300">{m.totp_enrollment_help()}</p>
					<TOTPSettings
						enrollmentToken={loginData.mfa_token}
						onEnrolled={() => void finishLogin({ access_token: auth.getToken() || '' })}
					/>
					<Button text={m.common_cancel()} variant="clear" click={goBackToLogin} />
				</div>
			{:else if !showPasswordChangeForm}
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
								{m.login_username_label()}
							</label>
							<input
								bind:this={usernameInput}
								id="username"
								bind:value={username}
								type="text"
								required
								onkeydown={handleUsernameKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder={m.login_username_placeholder()}
							/>
						</div>
						<div>
							<label
								for="password"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								{m.login_password_label()}
							</label>
							<input
								bind:this={passwordInput}
								id="password"
								bind:value={password}
								type="password"
								required
								onkeydown={handlePasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder={m.login_password_placeholder()}
							/>
						</div>
					</div>

					<Button
						class="w-full justify-center"
						type="submit"
						{loading}
						text={m.login_signin_button()}
						variant="filled"
					/>
				</form>

				<OAuthButtons />
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
								{m.login_new_password_label()}
							</label>
							<input
								bind:this={newPasswordInput}
								id="new-password"
								bind:value={newPassword}
								type="password"
								required
								onkeydown={handleNewPasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder={m.login_new_password_placeholder()}
							/>
						</div>
						<div>
							<label
								for="confirm-password"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								{m.login_confirm_password_label()}
							</label>
							<input
								bind:this={confirmPasswordInput}
								id="confirm-password"
								bind:value={confirmPassword}
								type="password"
								required
								onkeydown={handleConfirmPasswordKeydown}
								class="appearance-none rounded-lg block w-full px-4 py-3 border border-gray-300 dark:border-gray-600 placeholder-gray-400 dark:placeholder-gray-500 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent bg-white dark:bg-gray-700 transition-all"
								placeholder={m.login_confirm_password_placeholder()}
							/>
						</div>
					</div>

					<div class="space-y-3">
						<Button
							class="w-full justify-center"
							type="submit"
							{loading}
							text={m.login_change_password_button()}
							variant="filled"
						/>
						<Button
							class="w-full justify-center"
							text={m.login_back_to_login_button()}
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
