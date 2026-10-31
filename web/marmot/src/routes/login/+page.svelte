<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount, tick } from 'svelte';
	import OAuthButtons from '$components/auth/OAuthButtons.svelte';
	import { auth } from '$lib/stores/auth';
	import { userProfile } from '$lib/stores/user';
	import { m } from '$lib/paraglide/messages';
	import type { MmTextField } from '@marmotdata/elements/text-field.js';

	// The page is public, so it shows no hostname, version or build details: those help an attacker

	interface LoginResponse {
		access_token?: string;
		requires_password_change?: boolean;
	}

	interface PendingConsent {
		redirect_uri: string;
		client_id: string;
		client_name?: string;
		scopes: string[];
		scope_descriptions?: { scope: string; summary: string }[];
	}

	// Known scopes get translated copy; anything else falls back to the server's own description
	const scopeCopy: Record<string, () => string> = { openid: () => m.login_consent_scope_openid() };

	let username = $state('');
	let password = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);
	let showPasswordChangeForm = $state(false);
	let loginData: LoginResponse | null = $state(null);
	let pendingConsent = $state<PendingConsent | null>(null);
	let denied = $state(false);
	// The registered name is a claim the app makes about itself, so the id stays on screen beside it
	const consentClient = $derived(pendingConsent?.client_name || pendingConsent?.client_id || '');
	let usernameInput = $state<MmTextField>();
	let newPasswordInput = $state<MmTextField>();

	// The OAuth callback sends people here with ?error when the identity provider sign-in failed; the text is fixed so the URL cannot put words on the page
	const ssoFailed = page.url.searchParams.has('error');

	onMount(async () => {
		if (page.url.searchParams.has('oauth_pending')) {
			await loadPendingConsent();
		}
		if (!pendingConsent) {
			await customElements.whenDefined('mm-text-field');
			usernameInput?.focus();
		}
	});

	async function loadPendingConsent() {
		const token = auth.getToken();
		if (!token) return;
		// A stale token (expired, signing key rotated, server restarted) would show the consent screen and then fail at /oauth/authorize/complete with 401, so check it first
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
				userProfile.load();
			}
		} catch {
			/* pending consent fetch failed; remain on the sign-in screen */
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
			if (!response.ok) throw new Error(m.login_error_complete_authorization());
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
			await fetch('/oauth/authorize/cancel', { method: 'POST', credentials: 'same-origin' });
		} catch {
			/* best-effort cancel */
		}
		denied = true;
	}

	// Full document navigation so the account language preference in the token applies from the first render; only in-app paths are accepted so the redirect cannot leave Marmot
	function finishSignIn(token: string) {
		auth.setToken(token);
		const redirectParam = page.url.searchParams.get('redirect');
		const redirectTo =
			redirectParam && redirectParam.startsWith('/') && !redirectParam.startsWith('//')
				? redirectParam
				: '/';
		window.location.assign(resolve(redirectTo as '/'));
	}

	// Both a normal sign-in and a forced password change end here, so an OAuth client waiting for consent is asked either way rather than skipped after a password change
	async function completeSignIn(token: string) {
		if (page.url.searchParams.has('oauth_pending')) {
			auth.setToken(token);
			await loadPendingConsent();
			if (pendingConsent) return;
		}
		finishSignIn(token);
	}

	async function handleSubmit(e?: Event) {
		e?.preventDefault();
		if (loading) return;
		error = '';
		loading = true;
		try {
			const response = await fetch('/api/v1/users/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ username, password })
			});
			if (!response.ok) throw new Error(m.login_error_invalid_credentials());

			const data: LoginResponse = await response.json();
			loginData = data;

			if (data.requires_password_change) {
				showPasswordChangeForm = true;
				await tick();
				newPasswordInput?.focus();
				return;
			}
			if (!data.access_token) throw new Error(m.login_error_no_token());
			await completeSignIn(data.access_token);
		} catch (err) {
			error = err instanceof Error ? err.message : m.login_error_login_failed();
		} finally {
			loading = false;
		}
	}

	async function handlePasswordChange(e?: Event) {
		e?.preventDefault();
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
			const response = await fetch('/api/v1/users/update-password', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${loginData?.access_token ?? ''}`
				},
				body: JSON.stringify({ new_password: newPassword })
			});
			if (!response.ok) throw new Error(m.login_error_update_password());
			const passwordData = await response.json();
			if (!passwordData.access_token) throw new Error(m.login_error_no_token());
			await completeSignIn(passwordData.access_token);
		} catch (err) {
			error = err instanceof Error ? err.message : m.login_error_password_change_failed();
		} finally {
			loading = false;
		}
	}

	async function goBackToLogin() {
		showPasswordChangeForm = false;
		newPassword = '';
		confirmPassword = '';
		error = '';
		await tick();
		usernameInput?.focus();
	}

	function valueOf(e: Event): string {
		return (e.target as HTMLInputElement).value;
	}
</script>

<div class="sheet">
	<section class="brand" aria-label="Marmot">
		<mm-topography seed={5} fade="end"></mm-topography>
		<mm-logo size={36} clear-space></mm-logo>
	</section>

	<main class="side">
		<div class="mm-stack form">
			{#if pendingConsent && denied}
				<mm-heading level={1} variant="display-title">{m.login_denied_heading()}</mm-heading>
				<mm-text variant="body" tone="secondary" block
					>{m.login_denied_body({ client: consentClient })}</mm-text
				>
				<mm-button href={resolve('/')} icon="home">{m.login_go_home()}</mm-button>
			{:else if pendingConsent}
				<mm-heading level={1} variant="display-title">{m.login_authorize_heading()}</mm-heading>
				<mm-text variant="body" block>{m.login_consent_asking({ client: consentClient })}</mm-text>
				{#if $userProfile}
					<mm-owner-chip
						name={$userProfile.name || $userProfile.username}
						email={$userProfile.email ?? ''}
						avatar-size={24}
					></mm-owner-chip>
				{/if}
				{#if error}
					<mm-callout tone="danger" compact urgent>{error}</mm-callout>
				{/if}
				{#if pendingConsent.scope_descriptions?.length}
					<div class="mm-stack" style="--mm-gap: var(--space-2)">
						<mm-text variant="label" block>{m.login_consent_can({ client: consentClient })}</mm-text
						>
						<mm-list dense style="--mm-list-inset: 0">
							{#each pendingConsent.scope_descriptions as d (d.scope)}
								<mm-list-item
									><mm-icon slot="start" name="account-circle"></mm-icon>{scopeCopy[d.scope]?.() ??
										d.summary}</mm-list-item
								>
							{/each}
						</mm-list>
					</div>
				{/if}
				<mm-description-list layout="stacked">
					<mm-description term={m.login_consent_client()} mono
						>{pendingConsent.client_id}</mm-description
					>
					<mm-description term={m.login_consent_returns_to()} mono
						>{pendingConsent.redirect_uri}</mm-description
					>
				</mm-description-list>
				<mm-text variant="caption" tone="secondary" block
					>{m.login_consent_warning({ command: 'marmot login' })}</mm-text
				>
				<div class="mm-cluster" style="--mm-gap: var(--space-3)">
					<mm-button disabled={loading} onclick={handleConsentCancel}
						>{m.login_deny_button()}</mm-button
					>
					<mm-button variant="primary" disabled={loading} onclick={handleConsentAllow}
						>{m.login_authorize_button()}</mm-button
					>
				</div>
				<mm-text variant="caption" tone="secondary" block>{m.login_consent_revoke_hint()}</mm-text>
			{:else if showPasswordChangeForm}
				<mm-heading level={1} variant="display-title"
					>{m.login_change_password_heading()}</mm-heading
				>
				<mm-text variant="body" tone="secondary" block>{m.login_change_password_hint()}</mm-text>
				<form class="mm-stack fields" onsubmit={handlePasswordChange}>
					{#if error}
						<mm-callout tone="danger" compact urgent>{error}</mm-callout>
					{/if}
					<mm-text-field
						bind:this={newPasswordInput}
						name="new-password"
						type="password"
						label={m.login_new_password_label()}
						autocomplete="new-password"
						help={m.login_new_password_help()}
						value={newPassword}
						required
						oninput={(e: Event) => (newPassword = valueOf(e))}
					></mm-text-field>
					<mm-text-field
						name="confirm-password"
						type="password"
						label={m.login_confirm_password_label()}
						autocomplete="new-password"
						value={confirmPassword}
						required
						oninput={(e: Event) => (confirmPassword = valueOf(e))}
					></mm-text-field>
					<mm-button class="block" variant="primary" type="submit" disabled={loading}
						>{m.login_change_password_button()}</mm-button
					>
				</form>
				<mm-button variant="ghost" icon="arrow-back" onclick={goBackToLogin}
					>{m.login_back_to_login_button()}</mm-button
				>
			{:else}
				<mm-heading level={1} variant="display-title">{m.login_signin_heading()}</mm-heading>
				{#if ssoFailed}
					<mm-callout tone="danger" heading={m.login_sso_failed_heading()}
						>{m.login_sso_failed_body()}</mm-callout
					>
				{/if}
				<OAuthButtons />
				<form class="mm-stack fields" onsubmit={handleSubmit}>
					{#if error}
						<mm-callout tone="danger" compact urgent>{error}</mm-callout>
					{/if}
					<mm-text-field
						bind:this={usernameInput}
						name="username"
						label={m.login_username_label()}
						autocomplete="username"
						value={username}
						required
						oninput={(e: Event) => (username = valueOf(e))}
					></mm-text-field>
					<mm-text-field
						name="password"
						type="password"
						label={m.login_password_label()}
						autocomplete="current-password"
						value={password}
						required
						oninput={(e: Event) => (password = valueOf(e))}
					></mm-text-field>
					<mm-button class="block" variant="primary" type="submit" disabled={loading}
						>{m.login_signin_button()}</mm-button
					>
				</form>
				<mm-text variant="caption" tone="secondary" block>{m.login_forgotten_password()}</mm-text>
			{/if}
		</div>
	</main>
</div>

<style>
	.sheet {
		min-height: 100vh;
		background: var(--mm-paper);
		color: var(--mm-text);
		display: grid;
		grid-template-columns: minmax(0, 5fr) minmax(0, 4fr);
	}
	.brand {
		position: relative;
		isolation: isolate;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		gap: var(--space-6);
		padding: var(--space-8);
		background: var(--mm-sunken);
		border-inline-end: 1px solid var(--mm-rule);
	}
	.side {
		display: grid;
		align-items: center;
		justify-items: start;
		padding: var(--space-8) var(--space-12);
	}
	.form {
		--mm-gap: var(--space-5);
		inline-size: min(100%, 24rem);
	}
	.fields {
		--mm-gap: var(--space-4);
	}
	.block {
		display: flex;
	}
	@media (max-width: 768px) {
		.sheet {
			grid-template-columns: minmax(0, 1fr);
			grid-template-rows: auto 1fr;
		}
		.brand {
			min-height: 220px;
			padding: var(--space-5) var(--space-4);
			border-inline-end: 0;
			border-block-end: 1px solid var(--mm-rule);
		}
		.side {
			align-items: start;
			padding: var(--space-6) var(--space-4) var(--space-8);
		}
	}
</style>
